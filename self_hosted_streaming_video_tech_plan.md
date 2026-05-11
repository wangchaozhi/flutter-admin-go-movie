# 自建流媒体播放系统技术方案

## 1. 项目目标

本方案用于搭建一套早期可落地的自建视频播放系统，适合在暂时不接入 CDN、不使用云对象存储的情况下，完成视频上传、转码切片、播放、防盗链和后台管理。

技术栈：

- 后端：Go
- App 端：Flutter
- Admin 后台：React
- 存储：MinIO
- 转码切片：FFmpeg
- 视频访问层：Nginx
- 播放器：better_player_enhanced
- 数据库：PostgreSQL / MySQL
- 缓存与任务队列：Redis + Asynq

核心目标：

- 支持 MP4 上传
- 支持 FFmpeg 转 HLS
- 支持多清晰度播放
- 支持 m3u8 + ts 分片播放
- 支持视频防盗链
- 支持 Nginx 代理 MinIO
- 支持 Nginx 缓存热门视频分片
- Go 不直接承载大流量视频分片
- 后期可平滑迁移到 CDN / 云对象存储

---

## 2. 整体架构

```text
React Admin
   ↓ 上传 MP4
Go API
   ↓
MinIO：保存原始 MP4

Go Worker
   ↓ 调用 FFmpeg
生成 HLS：m3u8 + ts
   ↓
MinIO：保存 HLS 文件

Flutter App
   ↓ 请求播放
Go API：鉴权、会员判断、生成签名播放地址
   ↓ 返回 m3u8 URL

Flutter Player
   ↓ 请求 m3u8 / ts
Nginx：校验签名、防盗链、缓存、代理 MinIO
   ↓
MinIO
```

系统分工：

| 模块 | 职责 |
|---|---|
| React Admin | 视频上传、视频管理、转码状态、上下架 |
| Flutter App | 视频列表、详情、播放、观看记录 |
| Go API | 用户鉴权、视频业务、上传、播放授权、签名生成 |
| Go Worker | 异步转码、调用 FFmpeg、上传 HLS 到 MinIO |
| MinIO | 保存原始 MP4、HLS 切片、封面 |
| FFmpeg | MP4 转 HLS，多清晰度转码 |
| Nginx | 防盗链校验、代理 MinIO、缓存 ts 分片 |
| Redis / Asynq | 异步任务队列 |
| PostgreSQL / MySQL | 业务数据存储 |

---

## 3. 推荐技术选型

### 3.1 后端

推荐：

```text
Go + Gin + PostgreSQL + Redis + Asynq + MinIO SDK
```

可选框架：

- Gin
- Fiber
- Echo

推荐组合：

| 类型 | 推荐 |
|---|---|
| Web 框架 | Gin |
| ORM / SQL | GORM / sqlc |
| 数据库 | PostgreSQL |
| 缓存 | Redis |
| 队列 | Asynq |
| 存储 SDK | MinIO Go SDK |
| 转码 | FFmpeg |

---

### 3.2 Flutter 播放器

推荐使用：

```yaml
dependencies:
  better_player_enhanced: ^1.0.2
```

适合能力：

- HLS / m3u8 播放
- MP4 播放
- 全屏
- 进度拖动
- 播放控制栏
- 字幕
- 缓存
- PiP
- DASH / DRM 扩展能力

---

### 3.3 存储

使用 MinIO 自建对象存储。

建议：

- MinIO bucket 不公开
- 原始视频不允许用户直接访问
- HLS 文件也不允许用户直接访问
- 用户只能访问 Nginx 暴露的签名 URL

---

## 4. MinIO 存储设计

推荐 bucket：

```text
video
```

目录结构：

```text
video/
  originals/
    10001/
      source.mp4

  hls/
    10001/
      master.m3u8

      360p/
        index.m3u8
        seg_000.ts
        seg_001.ts

      720p/
        index.m3u8
        seg_000.ts
        seg_001.ts

      1080p/
        index.m3u8
        seg_000.ts
        seg_001.ts

  covers/
    10001/
      cover.jpg
```

访问原则：

```text
用户不能直接访问 MinIO
React Admin 不能直接暴露 MinIO 原始地址
Flutter App 不能直接访问 MinIO
Go 可以读写 MinIO
Nginx 可以只读代理 MinIO 的 HLS 文件
```

---

## 5. 视频处理流程

### 5.1 上传流程

```text
1. React Admin 创建视频记录
2. Admin 上传 MP4 到 Go API
3. Go API 保存 MP4 到 MinIO originals/
4. Go API 创建转码任务
5. Go Worker 异步执行转码
6. 转码完成后更新视频状态为 ready
```

MVP 阶段可以采用：

```text
Admin → Go → MinIO
```

后期可以升级为：

```text
Admin → MinIO 预签名上传
Go 只负责生成上传凭证和保存元数据
```

---

### 5.2 转码流程

```text
1. Go Worker 从 MinIO 下载 originals/{videoId}/source.mp4
2. 保存到本地临时目录
3. 调用 FFmpeg 生成 360p / 720p / 1080p HLS
4. 生成 master.m3u8
5. 上传 HLS 结果到 MinIO hls/{videoId}/
6. 更新 videos.status = ready
7. 删除本地临时文件
```

注意：

- 不要在 HTTP 请求里直接转码
- 转码必须异步执行
- 转码任务要限并发
- FFmpeg 很吃 CPU
- 同机部署时建议同时只跑 1 到 2 个转码任务

---

## 6. FFmpeg 转码切片方案

### 6.1 生成 360p

```bash
ffmpeg -i source.mp4 \
  -vf scale=-2:360 \
  -c:v libx264 -preset veryfast -crf 23 \
  -c:a aac -b:a 96k \
  -hls_time 6 \
  -hls_playlist_type vod \
  -hls_segment_filename "360p/seg_%03d.ts" \
  360p/index.m3u8
```

---

### 6.2 生成 720p

```bash
ffmpeg -i source.mp4 \
  -vf scale=-2:720 \
  -c:v libx264 -preset veryfast -crf 23 \
  -c:a aac -b:a 128k \
  -hls_time 6 \
  -hls_playlist_type vod \
  -hls_segment_filename "720p/seg_%03d.ts" \
  720p/index.m3u8
```

---

### 6.3 生成 1080p

```bash
ffmpeg -i source.mp4 \
  -vf scale=-2:1080 \
  -c:v libx264 -preset veryfast -crf 22 \
  -c:a aac -b:a 128k \
  -hls_time 6 \
  -hls_playlist_type vod \
  -hls_segment_filename "1080p/seg_%03d.ts" \
  1080p/index.m3u8
```

---

### 6.4 master.m3u8

```m3u8
#EXTM3U
#EXT-X-VERSION:3

#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360
360p/index.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720
720p/index.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080
1080p/index.m3u8
```

App 播放入口：

```text
https://api.example.com/api/hls/10001/master.m3u8?expires=xxx&sign=xxx
```

---

## 7. 防盗链设计

### 7.1 防盗链目标

防止用户直接复制视频地址长期访问，防止第三方盗刷流量。

防护目标：

- m3u8 不能长期有效
- ts 分片不能裸奔
- MinIO 不能公开访问
- 播放地址必须短时效
- 用户必须先通过 Go 后端鉴权
- 视频大流量不走 Go，走 Nginx

---

### 7.2 推荐防盗链方案

采用：

```text
Go 生成签名 URL
Nginx secure_link 校验
MinIO 私有 bucket
m3u8 动态改写
ts 分片走 Nginx
```

播放流程：

```text
Flutter 请求 /api/videos/:id/play
   ↓
Go 校验用户权限
   ↓
Go 返回 master.m3u8 签名地址
   ↓
Flutter 播放 m3u8
   ↓
Go 动态返回 m3u8，并改写里面的子 m3u8 / ts 地址
   ↓
ts 分片请求 video.example.com/hls/...
   ↓
Nginx 校验 sign / expires
   ↓
Nginx 代理 MinIO 返回 ts
```

---

### 7.3 签名规则

建议 MVP 阶段使用：

```text
sign = md5(expires + uri + secret)
```

示例：

```text
uri = /hls/10001/720p/seg_001.ts
expires = 1710000000
secret = your_secret_key
raw = 1710000000/hls/10001/720p/seg_001.tsyour_secret_key
sign = md5(raw)
```

最终地址：

```text
https://video.example.com/hls/10001/720p/seg_001.ts?expires=1710000000&sign=xxxxxx
```

建议有效期：

| 场景 | 有效期 |
|---|---|
| 普通视频 | 10 到 30 分钟 |
| 试看视频 | 1 到 5 分钟 |
| VIP 视频 | 5 到 15 分钟 |
| 直播 | 30 秒到 2 分钟 |

---

## 8. Go 签名生成示例

```go
package video

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"
)

func SignPath(path string, secret string, ttlSeconds int64) string {
	expires := time.Now().Unix() + ttlSeconds

	raw := fmt.Sprintf("%d%s%s", expires, path, secret)
	sum := md5.Sum([]byte(raw))
	sign := hex.EncodeToString(sum[:])

	return fmt.Sprintf("%s?expires=%d&sign=%s", path, expires, sign)
}
```

示例：

```go
url := SignPath("/hls/10001/720p/seg_001.ts", "your_secret_key", 1800)
```

输出：

```text
/hls/10001/720p/seg_001.ts?expires=1710000000&sign=xxxxxx
```

---

## 9. m3u8 动态改写

### 9.1 为什么要改写 m3u8

如果 `index.m3u8` 里是：

```m3u8
#EXTINF:6.0,
seg_001.ts
#EXTINF:6.0,
seg_002.ts
```

播放器请求分片时会访问：

```text
/hls/10001/720p/seg_001.ts
/hls/10001/720p/seg_002.ts
```

这些地址没有签名，会被 Nginx 拒绝。

因此需要把 m3u8 里的分片地址改写成带签名的 URL。

---

### 9.2 master.m3u8 改写示例

原始内容：

```m3u8
#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360
360p/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720
720p/index.m3u8
```

Go 动态返回：

```m3u8
#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360
https://api.example.com/api/hls/10001/360p/index.m3u8?expires=xxx&sign=xxx
#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720
https://api.example.com/api/hls/10001/720p/index.m3u8?expires=xxx&sign=xxx
```

---

### 9.3 index.m3u8 改写示例

原始内容：

```m3u8
#EXTINF:6.0,
seg_000.ts
#EXTINF:6.0,
seg_001.ts
```

Go 动态返回：

```m3u8
#EXTINF:6.0,
https://video.example.com/hls/10001/720p/seg_000.ts?expires=xxx&sign=xxx
#EXTINF:6.0,
https://video.example.com/hls/10001/720p/seg_001.ts?expires=xxx&sign=xxx
```

这样可以实现：

```text
m3u8 小文件经过 Go
ts 大文件经过 Nginx
```

---

## 10. Nginx 配置

### 10.1 Nginx 职责

Nginx 负责：

- 校验 `sign`
- 校验 `expires`
- 拒绝过期请求
- 代理 MinIO
- 缓存 ts 分片
- 支持 Range 请求
- 支持高并发视频分片请求

---

### 10.2 Nginx 配置示例

```nginx
proxy_cache_path /data/nginx/video_cache
    levels=1:2
    keys_zone=video_cache:500m
    max_size=100g
    inactive=7d
    use_temp_path=off;

server {
    listen 443 ssl http2;
    server_name video.example.com;

    ssl_certificate /etc/nginx/ssl/video.example.com.crt;
    ssl_certificate_key /etc/nginx/ssl/video.example.com.key;

    location /hls/ {
        secure_link $arg_sign,$arg_expires;
        secure_link_md5 "$secure_link_expires$uri your_secret_key";

        if ($secure_link = "") {
            return 403;
        }

        if ($secure_link = "0") {
            return 410;
        }

        proxy_pass http://minio:9000/video/hls/;
        proxy_set_header Host minio:9000;

        proxy_buffering on;
        proxy_request_buffering off;

        proxy_cache video_cache;
        proxy_cache_valid 200 206 7d;
        proxy_cache_lock on;

        proxy_set_header Range $http_range;
        proxy_set_header If-Range $http_if_range;

        add_header Access-Control-Allow-Origin *;
        add_header Access-Control-Allow-Methods "GET, HEAD, OPTIONS";
        add_header Access-Control-Allow-Headers "*";

        types {
            application/vnd.apple.mpegurl m3u8;
            video/mp2t ts;
            video/mp4 mp4;
        }
    }
}
```

注意：

- `/hls/` 主要用于承载 `.ts` 分片
- 如果直接让 Nginx 返回 `.m3u8`，必须解决分片签名继承问题
- 推荐让 Go 动态返回 m3u8，让 Nginx 专注承载大流量 ts 分片

---

## 11. 数据库设计

### 11.1 videos 表

```sql
CREATE TABLE videos (
  id BIGSERIAL PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  description TEXT,
  cover_key TEXT,

  original_key TEXT,
  hls_master_key TEXT,

  duration INT DEFAULT 0,
  size BIGINT DEFAULT 0,

  status VARCHAR(32) NOT NULL DEFAULT 'uploading',
  is_vip BOOLEAN DEFAULT FALSE,
  is_free BOOLEAN DEFAULT TRUE,

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

状态说明：

| 状态 | 含义 |
|---|---|
| uploading | 上传中 |
| uploaded | 上传完成 |
| transcoding | 转码中 |
| ready | 可播放 |
| failed | 转码失败 |
| offline | 已下架 |

---

### 11.2 video_transcode_tasks 表

```sql
CREATE TABLE video_transcode_tasks (
  id BIGSERIAL PRIMARY KEY,
  video_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  error_message TEXT,

  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

状态说明：

| 状态 | 含义 |
|---|---|
| pending | 等待处理 |
| processing | 处理中 |
| success | 成功 |
| failed | 失败 |

---

### 11.3 video_play_records 表

```sql
CREATE TABLE video_play_records (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  video_id BIGINT NOT NULL,
  position INT DEFAULT 0,
  duration INT DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## 12. 接口设计

### 12.1 Admin 创建视频

```http
POST /admin/videos
```

请求：

```json
{
  "title": "测试视频",
  "description": "视频简介",
  "is_vip": false,
  "is_free": true
}
```

---

### 12.2 Admin 上传 MP4

```http
POST /admin/videos/{videoId}/upload
Content-Type: multipart/form-data
```

表单字段：

```text
file: source.mp4
```

---

### 12.3 Admin 开始转码

```http
POST /admin/videos/{videoId}/transcode
```

响应：

```json
{
  "task_id": 123,
  "status": "pending"
}
```

---

### 12.4 Admin 查询转码状态

```http
GET /admin/videos/{videoId}/transcode
```

响应：

```json
{
  "video_id": 10001,
  "status": "processing",
  "error_message": ""
}
```

---

### 12.5 App 获取视频列表

```http
GET /api/videos
```

---

### 12.6 App 获取视频详情

```http
GET /api/videos/{videoId}
```

---

### 12.7 App 获取播放地址

```http
GET /api/videos/{videoId}/play
Authorization: Bearer 用户Token
```

响应：

```json
{
  "video_id": 10001,
  "type": "hls",
  "url": "https://api.example.com/api/hls/10001/master.m3u8?expires=1710000000&sign=xxxx"
}
```

---

### 12.8 Go 动态返回 m3u8

```http
GET /api/hls/{videoId}/master.m3u8?expires=xxx&sign=xxx
GET /api/hls/{videoId}/{quality}/index.m3u8?expires=xxx&sign=xxx
```

返回：

```text
Content-Type: application/vnd.apple.mpegurl
```

---

### 12.9 Nginx 返回 ts 分片

```http
GET https://video.example.com/hls/{videoId}/{quality}/seg_000.ts?expires=xxx&sign=xxx
```

---

## 13. Flutter 播放示例

```dart
final dataSource = BetterPlayerDataSource(
  BetterPlayerDataSourceType.network,
  playUrl,
  videoFormat: BetterPlayerVideoFormat.hls,
);

final controller = BetterPlayerController(
  const BetterPlayerConfiguration(
    autoPlay: true,
    aspectRatio: 16 / 9,
    fit: BoxFit.contain,
    controlsConfiguration: BetterPlayerControlsConfiguration(
      enableFullscreen: true,
      enableProgressText: true,
      enableProgressBar: true,
      enableSkips: true,
      enablePlayPause: true,
    ),
  ),
  betterPlayerDataSource: dataSource,
);
```

播放流程：

```text
1. Flutter 请求 /api/videos/{videoId}/play
2. Go 返回 m3u8 播放地址
3. better_player_enhanced 播放 m3u8
4. m3u8 中的 ts 分片请求 Nginx
5. Nginx 校验签名后代理 MinIO
```

---

## 14. React Admin 功能规划

后台页面：

```text
/videos
/videos/create
/videos/{id}/edit
/videos/{id}/transcode
```

基础功能：

- 登录
- 视频列表
- 创建视频
- 上传 MP4
- 上传封面
- 编辑标题
- 编辑简介
- 是否 VIP
- 是否免费
- 上架 / 下架
- 删除视频
- 查看转码状态
- 重新转码
- 查看播放地址
- 查看视频元数据

---

## 15. 部署方案

### 15.1 Docker Compose 服务

推荐服务：

```text
go-api
go-worker
postgres
redis
minio
nginx
```

项目目录：

```text
project/
  backend/
  admin/
  app/
  deploy/
    docker-compose.yml
    nginx/
      nginx.conf
    minio/
    postgres/
```

---

### 15.2 服务器配置建议

MVP 阶段：

| 配置 | 建议 |
|---|---|
| CPU | 4 核起，推荐 8 核 |
| 内存 | 8G 起，推荐 16G |
| 磁盘 | SSD 500G 起 |
| 带宽 | 20Mbps 起，推荐 50Mbps+ |
| 系统 | Ubuntu 22.04 / Debian 12 |

如果转码和播放部署在同一台机器：

- 限制 FFmpeg 并发
- 建议同时只跑 1 到 2 个转码任务
- 避免转码占满 CPU 影响播放
- Nginx 缓存目录单独放到大容量磁盘

---

## 16. Content-Type 设置

上传到 MinIO 时建议设置正确类型：

| 后缀 | Content-Type |
|---|---|
| `.m3u8` | `application/vnd.apple.mpegurl` |
| `.ts` | `video/mp2t` |
| `.mp4` | `video/mp4` |
| `.jpg` | `image/jpeg` |
| `.webp` | `image/webp` |

---

## 17. 安全策略

### 17.1 必做

- MinIO bucket 私有
- Go 播放接口必须登录鉴权
- VIP 视频必须校验会员或购买状态
- m3u8 地址短时效
- ts 分片必须带签名
- Nginx 校验 sign 和 expires
- Nginx 限制异常访问
- 后台接口必须有 Admin 权限
- 原始 MP4 不对用户暴露

---

### 17.2 建议做

- 播放地址绑定 user_id
- 播放地址绑定 video_id
- 可选绑定 IP
- 可选绑定 User-Agent
- 记录播放日志
- 记录异常请求
- 对异常 IP 限流
- 视频试看限制
- 设备数限制
- 登录态过期控制

---

## 18. Nginx 缓存策略

由于当前阶段不使用 CDN，Nginx 缓存可以当作一个轻量本地 CDN。

建议：

```text
.ts 分片缓存 7 天
.m3u8 不建议由 Nginx 静态缓存
热门分片命中 Nginx cache
减少 MinIO 压力
```

缓存配置：

```nginx
proxy_cache video_cache;
proxy_cache_valid 200 206 7d;
proxy_cache_lock on;
```

---

## 19. 上线阶段路线

### 第一阶段：MVP

目标：跑通完整播放链路。

实现：

- Admin 上传 MP4
- Go 保存 MinIO
- FFmpeg 生成 720p HLS
- Go 动态返回 m3u8
- Nginx 校验 ts 分片
- Flutter 播放 HLS

只做一个清晰度：

```text
720p
```

---

### 第二阶段：多清晰度

增加：

- 360p
- 720p
- 1080p
- master.m3u8
- 自动清晰度切换

---

### 第三阶段：增强防护

增加：

- 短时效签名
- 试看限制
- IP 限制
- 设备数限制
- 播放记录
- 异常请求限流
- Nginx proxy_cache
- 后台转码日志

---

### 第四阶段：接入 CDN

当用户量上来后，将：

```text
Flutter → Nginx → MinIO
```

升级为：

```text
Flutter → CDN → Nginx / MinIO
```

Go 的播放鉴权和签名逻辑基本不用推倒重来。

---

## 20. 最终推荐架构总结

当前阶段推荐：

```text
React Admin 上传 MP4
Go 保存到 MinIO
Go Worker 调 FFmpeg 转 HLS
HLS 文件存 MinIO
Go 鉴权后返回 m3u8 播放入口
Go 动态改写 m3u8，给子 m3u8 和 ts 加签名
Nginx secure_link 校验 ts 分片
Nginx proxy_cache 缓存热门分片
Flutter 用 better_player_enhanced 播放
```

一句话总结：

```text
Go 管业务和 m3u8 签名，Nginx 管大流量视频分片，MinIO 管存储，FFmpeg 管切片，Flutter 管播放，React Admin 管内容。
```

该方案适合先自建跑起来，后期迁移到 CDN 或云对象存储时，整体架构可以平滑升级。
