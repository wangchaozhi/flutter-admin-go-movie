# 播放器与 HLS 链路优化记录

本文档记录本次针对 Flutter 移动端播放器、Go HLS 接口、FFmpeg 转码链路和 Nginx 分片缓存的优化内容。

## 优化目标

- 提升长视频播放稳定性，避免签名过期导致中途卡死。
- 支持断点续播和可靠进度保存。
- 改善清晰度切换、缓冲、错误恢复、全屏和倍速等播放器体验。
- 优化转码档位与 HLS 切片参数，减少不必要的存储和带宽消耗。
- 提升 Nginx 分片缓存命中率，并避免客户端缓存过期 m3u8。
- 将 HLS 签名密钥从 Nginx 静态配置中移出，改为环境变量注入。

## 移动端播放器

主要文件：

- `front/mobile/lib/features/video/video_player_page.dart`

### 断点续播

播放器进入页面后会调用：

```text
GET /api/videos/{id}/progress
```

读取上次播放位置。媒体打开并获得 duration 后，如果记录位置有效且不接近片尾，会自动 seek 到该位置。

播放中每 10 秒保存一次进度：

```text
POST /api/videos/{id}/progress
```

页面退出、App 进入后台、暂停、隐藏或 detached 时，也会强制保存一次进度。

### 播放完成处理

监听 `media_kit` 的 `completed` 流：

- 播放完成后将后端进度写回 `0`。
- 页面显示“重播”按钮。
- 再次播放时从头开始，避免下次从片尾附近续播。

### 播放地址续签与错误恢复

播放器会定时刷新：

```text
GET /api/videos/{id}/play
```

刷新周期为 20 分钟。刷新时会保留当前清晰度和播放位置。

同时监听播放器错误流。当错误信息包含 `403`、`410`、`expired` 或 `forbidden` 时，会自动重新拉取播放地址并续播，覆盖 HLS 签名过期场景。

### 清晰度切换

清晰度切换仍基于重新打开对应 m3u8 后 seek 回原位置，但做了以下保护：

- 切换期间显示 loading 状态。
- 不再提前修改 `_selectedQuality`，只有切换成功才更新当前清晰度。
- 切换失败后自动尝试恢复原清晰度。
- 切换后保留当前倍速。

### 播放器交互

新增或增强了以下体验：

- 缓冲状态 spinner。
- 错误页“重试”按钮。
- 快退 10 秒。
- 快进 10 秒。
- 倍速菜单：`0.75x`、`1x`、`1.25x`、`1.5x`、`2x`。
- 全屏/退出全屏。
- 全屏时进入横屏沉浸式 UI。
- 全屏下返回键优先退出全屏。
- 退出页面时恢复竖屏和系统 UI。

## Go HLS 接口

主要文件：

- `backend/internal/video/hls_handler.go`

### HLS 签名有效期

原先多处使用 `1800` 秒硬编码。现在统一为：

```go
const hlsSignedURLTTLSeconds = 6 * 60 * 60
```

覆盖：

- master m3u8 地址。
- 子清晰度 index m3u8 地址。
- `.ts` 分片地址。
- `/api/videos/{id}/play` 返回的清晰度列表。

### m3u8 缓存策略

Go 返回的 master m3u8 和 index m3u8 都设置：

```http
Cache-Control: no-store
```

这样客户端不会缓存包含旧签名的播放列表，避免分片签名过期后仍继续使用旧 m3u8。

### 播放进度保护

保存进度时增加入参保护：

- position 小于 0 时归零。
- duration 小于 0 时归零。
- position 大于 duration 时截断到 duration。

## FFmpeg 转码链路

主要文件：

- `backend/internal/video/worker.go`
- `backend/internal/video/worker_test.go`

### 新增 480p 档位

当前转码档位：

```text
360p
480p
720p
1080p
```

480p 对移动网络更友好，比 360p 清晰，同时比 720p 更省带宽。

### 避免低清片源被放大

转码前通过 `ffprobe` 探测源视频宽高：

```text
stream=width,height
```

只生成不高于源视频高度的清晰度档位。例如：

- 720p 源不会再生成 1080p。
- 480p 源不会生成 720p 和 1080p。
- 低于 360p 的源视频，会按源高度生成一档，避免无意义放大。

### 动态 RESOLUTION

master playlist 中的 `RESOLUTION` 不再固定假设 16:9，而是按源视频比例动态计算。

这能正确支持：

- 横屏 16:9。
- 竖屏视频。
- 4:3 视频。
- 电影宽屏视频。

新增测试覆盖：

- 横屏 1920x1080。
- 竖屏 1080x1920。
- 低于 360p 的 426x240。

### HLS 切片与关键帧

转码参数增加：

```text
-g 180
-keyint_min 180
-force_key_frames expr:gte(t,n_forced*6)
-sc_threshold 0
-hls_flags independent_segments
```

目标：

- 每 6 秒强制关键帧。
- 降低拖动和清晰度切换时的等待。
- 让 HLS 分片更独立、更适合缓存和播放。

## Nginx 分片缓存

主要文件：

- `deploy/nginx/nginx.conf`
- `deploy/nginx/Dockerfile`
- `docker-compose.yml`

### 分片缓存 key

Nginx `/hls/` 分片代理增加：

```nginx
proxy_cache_key $uri;
```

签名参数 `expires` 和 `sign` 不再进入缓存 key。同一个 `.ts` 分片即使多次续签，也会命中同一份缓存对象。

### 分片缓存响应头

`.ts` 分片响应增加：

```http
Cache-Control: public, max-age=604800, immutable
```

Nginx 仍保留：

```nginx
proxy_cache_valid 200 206 7d;
proxy_cache_lock on;
proxy_cache_use_stale error timeout updating;
```

### HLS_SECRET 环境变量注入

Nginx 配置中的签名密钥不再硬编码：

```nginx
secure_link_md5 "$secure_link_expires$uri ${HLS_SECRET}";
```

Dockerfile 改为使用官方 Nginx 镜像模板机制：

```dockerfile
COPY nginx.conf /etc/nginx/templates/default.conf.template
```

`docker-compose.yml` 为 Nginx 注入：

```yaml
environment:
  HLS_SECRET: ${HLS_SECRET:-your_hls_secret_key_change_in_prod}
```

生产环境应显式设置 `HLS_SECRET`，并确保 Go 后端使用同一个 `HLS_SECRET`。

## 验证命令

本次优化执行过以下验证：

```bash
cd front/mobile
dart format lib/features/video/video_player_page.dart
flutter analyze
```

```bash
cd backend
gofmt -w internal/video/hls_handler.go internal/video/worker.go internal/video/worker_test.go
go test ./...
```

```bash
cd front/admin
npm run build
```

```bash
docker compose config
docker build -t flutter-admin-go-movie-nginx-test ./deploy/nginx
docker run --rm --add-host minio:127.0.0.1 \
  -e HLS_SECRET=your_hls_secret_key_change_in_prod \
  flutter-admin-go-movie-nginx-test nginx -t
```

## 部署注意事项

- 已经转码完成的旧视频不会自动拥有 480p、新的 GOP 参数或动态 `RESOLUTION`，需要重新转码后生效。
- Nginx 模板化后，修改 `HLS_SECRET` 需要重建或重启 Nginx 容器，让模板重新渲染。
- Go 后端和 Nginx 必须使用相同的 `HLS_SECRET`，否则分片校验会返回 403。
- Nginx 分片缓存按 `$uri` 命中。鉴权仍由签名校验保证，缓存 key 不包含签名参数不会绕过 `secure_link` 校验。
- 开发环境默认 `HLS_SECRET` 仍有 fallback 值，生产环境必须覆盖。

