# flutter-admin-go

一个包含 Go 后端、React 管理端和 Flutter 移动端的全栈示例项目。后端提供登录、用户、角色、菜单和按钮权限接口，使用 PostgreSQL 存储数据，数据访问层基于 GORM。

## 项目结构

```text
.
├── docker-compose.yml                 # PostgreSQL + MinIO 本地开发环境
├── docs/                              # 项目文档
├── backend/                           # Go 后端模块
│   ├── go.mod / go.sum                # 后端依赖
│   ├── cmd/server/main.go             # Go 服务入口
│   └── internal/
│       ├── admin/                     # 管理端接口
│       ├── auth/                      # 登录接口
│       ├── server/                    # 路由和 CORS
│       └── store/                     # GORM、PostgreSQL、SQL 迁移
│           └── migrations/            # 模块化 SQL 迁移文件
├── front/admin/                       # React + TypeScript + Vite 管理端
└── front/mobile/                      # Flutter 移动端
```

## 文档

- [播放器与 HLS 链路优化记录](docs/player_optimization.md)

## 功能概览

- 管理端 RBAC：管理员、角色、菜单和按钮权限管理，主界面菜单会根据 `admin_menus` 的父子关系、排序和图标编码动态渲染。
- 仪表盘：登录后默认进入「仪表盘」，展示视频/类别/用户/订单总量、各状态分布、已支付收入（按币种）和热门视频（按播放量）。
- 内容管理：视频、类别、影片资料（演员、导演、地区、年份、语言、类型）、转码任务、转码历史记录和多清晰度 HLS 资源管理。
- 剧集/选集：支持电视剧/动漫/综艺等**多集内容**。「一集即一个视频」——`series` 表只做分组和元信息，分集复用 `videos` 表（带 `series_id`/`episode_number`），因此转码、HLS、VIP 试看、评论、弹幕、播放进度等能力对每一集自动生效。管理端「剧集」菜单可创建剧集、上传封面、从未分配视频中挑选并编号为分集（`series:create/edit/delete`）；移动端首页有「热门剧集」横向 rail，点进剧集详情页可选集播放。分集视频不会再出现在 App 的扁平视频列表里，只通过剧集入口浏览。
- AI 元信息补全：管理端可调用 DeepSeek / OpenAI-compatible API 自动生成视频简介、看点和标签；移动端播放页展示简介、看点、标签和结构化影片资料。
- App 管理：移动端用户资料、状态和登录密码维护。
- 支付管理：会员套餐、单片套餐、订单列表、订单删除和**订单退款**（退款会回收会员套餐对应的 VIP 天数并把订单置为 `refunded`），订单列表会展示 App 用户名。
- 真实支付链路（模块化,均为原生 HTTP,无第三方 SDK）：每个网关一个文件,共享 `crypto.go`（RSA 签名/验签、密钥解析、AES-GCM 解密）和幂等处理 `events.go`;回调由统一分发器 `POST /api/webhooks/{provider}`（见 `payment/webhook.go` 的 `webhookRegistry`）路由,新增网关只需加一个实现文件 + 注册表一行。已接入四个渠道:
  - **Stripe**：Checkout Session 下单 + 退款;回调校验 `Stripe-Signature`（HMAC-SHA256 + 时间戳容差）。
  - **PayPal**：Orders v2 下单 + 捕获 + 退款;回调调用 `verify-webhook-signature` 验签。
  - **微信支付 v3**：Native（扫码 `code_url`）下单 + 退款;请求用商户私钥 RSA 签名,回调用 APIv3 密钥 AES-256-GCM 认证解密。
  - **支付宝**：`alipay.trade.precreate`（扫码 `qr_code`）下单 + `alipay.trade.refund` 退款;请求 RSA2 签名,异步通知用支付宝公钥验签。
  - 统一以 `payment_events`（`UNIQUE(provider,event_id)`）+ `markOrderPaid` 行锁做**幂等**,确认到账后发放 VIP;未配置密钥时仍可用 `mock` 渠道走通本地流程。移动端 VIP 页可选 模拟/Stripe/PayPal/微信/支付宝 五种渠道。
- 移动端账号:支持自助**注册**（`POST /api/mobile/register`,用户名/密码/可选昵称邮箱,带按 IP 限流,注册成功自动登录）和登录后**修改密码**（`PUT /api/mobile/password`,校验当前密码;无邮件设施,暂不提供邮箱找回）。
- 移动端：视频浏览、搜索（带本地搜索历史）、播放、收藏、观看历史、个人设置、商品和订单。
- 评论与评分：移动端播放页可发表评论和 1–5 星评分、查看平均分和他人评论、删除自己的评论；管理端「评论」菜单可搜索并删除违规评论（`comment:delete`）。**每个用户对同一视频只保留一条评分**（数据库 `(video_id, user_id) WHERE rating > 0` 部分唯一索引 + upsert，重复评分自动覆盖，平均分不再被刷）；评论/评分发表带**按用户限流**（默认每分钟 10 次）防刷。
- 弹幕：移动端播放页支持发送和展示**弹幕**（点播弹幕，按 `time_ms` 锚定到播放进度回放），支持滚动/顶部/底部三种模式和颜色选择；弹幕轨道用 Ticker + CustomPainter 渲染，暂停时冻结、seek 时重新对齐，并做轨道防重叠。发送带**按用户限流**（默认每分钟 20 条）。后端表 `video_danmaku` 按 `(video_id, time_ms)` 建索引；点播场景无需消息队列/实时广播。
  - 弹幕互动（参考主流影视平台）：可**点赞**任意弹幕、**删除自己**发的弹幕。点赞用 `danmaku_likes` 表（`UNIQUE(danmaku_id,user_id)` 保证幂等）+ `video_danmaku.like_count` 反范式计数，在事务里同步；有点赞的弹幕在轨道上内联显示 `♥N`。两种交互入口：①**直接点中飞动的弹幕**——自定义 hitTest 做到「命中才拦截」：点到弹幕就把它**定住**并浮出操作卡（点赞 ❤数 / 删除自己的），**视频继续播、其他弹幕继续飞**；没点中则穿透给播放器照常显隐控制条，二者互不干扰。②播放器的「弹幕列表」面板，逐条点赞/删除。
- 操作审计：后台所有写操作（`/api/admin` 下的 POST/PUT/DELETE）异步写入 `audit_logs`，记录执行人、方法、路径、状态码、IP 和 `request_id`；管理端「审计日志」菜单可搜索（按管理员/路径）和分页查看。
- 数据导出：管理端订单列表支持「导出 CSV」（`GET /api/admin/orders?format=csv`，带 UTF-8 BOM，便于财务对账）。仪表盘新增「收入趋势」近 30 天按主货币的每日已支付收入柱状图。
- 视频搜索：管理端视频列表支持按标题/ID 关键字和类别筛选；App 与管理端的视频列表接口均支持 `q` 关键字（标题，忽略大小写）、`category_id`、分页等查询参数。
- 首页推荐：`GET /api/home` 聚合「热门（按播放量）/ 最新上架 / VIP 精选」三条横向推荐 rail，移动端首页「全部」频道展示；公开 rail 带 Redis 短缓存（30s），认证可选——携带移动端 token 时额外返回个性化「继续观看」rail（最近未看完、≥95% 视为已看完的视频，带观看进度条）。
- 会员生命周期：`/api/mobile/profile` 返回 `days_remaining`（剩余天数）；移动端 VIP 页展示会员有效期、剩余天数与「即将到期」提醒（≤7 天）。后端有订单过期清扫任务（`payment.StartOrderExpiryJanitor`），定期把超过 `expires_at` 仍未支付的 `pending`/`paying` 订单置为 `cancelled`。会员到期按 `vip_until` 在读取时即时判定，无需额外降级任务。

## 环境要求

- Docker / Docker Compose
- Go 1.26+
- Node.js 和 npm
- Flutter SDK 3.10+

## 启动 PostgreSQL 和 MinIO

```bash
docker compose up -d postgres minio
```

PostgreSQL 默认连接信息：

```text
host=localhost
port=5432
database=flutter_admin_go
user=admin_go
password=admin_go_password
```

后端默认会使用上面的连接。也可以通过 `DATABASE_DSN` 覆盖：

```bash
cd backend
DATABASE_DSN="host=localhost port=5432 user=admin_go password=admin_go_password dbname=flutter_admin_go sslmode=disable TimeZone=Asia/Shanghai" go run ./cmd/server
```

MinIO 默认信息：

```text
API:     http://localhost:9000
Console: http://localhost:9001
user:    admin_go
password: admin_go_password
bucket:  admin-avatars
```

也可以通过环境变量覆盖：

```text
MINIO_ENDPOINT
MINIO_ACCESS_KEY
MINIO_SECRET_KEY
MINIO_USE_SSL
MINIO_AVATAR_BUCKET
```

## 启动后端

```bash
cd backend
go mod download
go run ./cmd/server
```

服务默认运行在：

```text
http://localhost:8080
```

后端配置文件在 `backend/config/*.yml`，配置读取逻辑集中在 `backend/internal/config`。通过 `APP_ENV` 选择环境：

```bash
APP_ENV=local go run ./cmd/server # 默认，本机 PostgreSQL/Redis/MinIO
APP_ENV=dev go run ./cmd/server   # Docker Compose 服务名 postgres/redis/minio
APP_ENV=prod go run ./cmd/server  # 生产环境，需通过环境变量补齐连接和密钥
```

配置仍可用环境变量覆盖，例如 `HTTP_ADDR`、`DATABASE_DSN`、`REDIS_ADDR`、`MINIO_ENDPOINT`、`JWT_SECRET`、`HLS_SECRET`。
转码 worker 默认 `TRANSCODE_VIDEO_ENCODER=auto`，会按系统候选和实际可用性选择 GPU 编码器，不可用时回退 `libx264`。也可以显式设置 `TRANSCODE_VIDEO_ENCODER=libx264` 强制使用 CPU；可选值包括 `h264_nvenc`、`h264_qsv`、`h264_vaapi`、`h264_videotoolbox`、`h264_amf`。

首次启动会自动执行 `backend/internal/store/migrations/*.sql`。已执行版本记录在 `schema_migrations` 表中。
后端也会自动创建头像 bucket。用户主题、头像对象 key 和缩略图对象 key 由迁移文件写入 `admin_users` 扩展字段。

健康检查会探测关键依赖（PostgreSQL、MinIO，以及配置了 Redis 时的 Redis），任一**必需**依赖不可用时返回 `503`，便于编排平台据此做就绪/存活判断；Redis 视为可选，不影响整体健康：

```text
GET /api/health   # 返回 { status, dependencies: { database, object_store, redis } }
```

## 可观测性

后端在最外层包了一层中间件（`internal/server/middleware.go`）：

- 为每个请求生成 `request_id` 并通过响应头 `X-Request-ID` 返回，同时写入 `context`（`server.RequestID(ctx)` 可取用）。
- 捕获任意层 panic，返回 `500` 而不是中断连接，并记录 `error` 级日志。
- 用 `log/slog` 打一条结构化访问日志（method、path、status、bytes、ip、duration）；`4xx` 记 `warn`、`5xx` 记 `error`、`/api/health` 记 `debug`。

日志格式按环境切换：`local`/`dev` 输出彩色文本到 stderr，`prod` 输出 JSON 到 stdout。可用 `LOG_LEVEL=debug|info|warn|error` 覆盖级别。

## AI 视频信息补全

管理端视频列表和编辑面板提供「AI补全」按钮，用于生成并缓存播放页信息：

- `synopsis`：主简介，会在原简介为空时自动写回 `videos.description`。
- `highlights`：播放页「看点」列表。
- `tags`：播放页标签。

生成结果保存在 `video_ai_metadata` 表。视频自身的结构化影片资料保存在 `videos` 表，包括：

```text
actors        # 演员，JSON 数组
directors     # 导演，JSON 数组
genres        # 类型，JSON 数组
region        # 地区
release_year  # 年份
language      # 语言
```

AI provider 目前按 OpenAI-compatible Chat Completions 抽象，默认配置指向 DeepSeek：

```bash
AI_ENABLED=true
AI_PROVIDER=deepseek
AI_API_KEY="<your-api-key>"
AI_BASE_URL="https://api.deepseek.com"
AI_MODEL="deepseek-v4-flash"
AI_TIMEOUT_SECONDS=45
```

不要把真实 API key 写入 `backend/config/*.yml` 或提交到仓库；请使用环境变量或部署平台 Secret。AI 只会根据标题、已有简介、分类、演职员、地区、年份、类型、语言、时长和分辨率等元数据生成内容；演员、导演、地区、年份、语言为空时提示词要求模型不要编造。

## 启动管理端

```bash
cd front/admin
npm install
npm run dev
```

管理端默认账号：

```text
admin / 123456
operator / 123456
```

`admin` 拥有全部菜单和按钮权限。已植入并由后端校验的按钮权限包括：

```text
user:create / user:edit / user:delete
role:create / role:edit / role:delete
menu:create / menu:edit / menu:delete
app_user:create / app_user:edit / app_user:delete
category:create / category:edit / category:delete
series:create / series:edit / series:delete
payment:product / payment:order / payment:refund
video:create / video:edit / video:delete
video:transcode-history    # 转码历史删除
video:extract-history      # 提取历史删除
```

这些权限**在后端按 HTTP 方法逐一校验**（见 `internal/server/router.go` 的 `requirePerm` 与 `admin.EnsurePermission`），不再依赖前端隐藏按钮：缺少权限的管理员即使直接调用接口，写操作也会返回 `403`。GET 读接口对任意已登录管理员开放。视频的上传 / 封面 / AI 补全 / 转码归类为「编辑」，需要 `video:edit`；删除视频或转码产物需要 `video:delete`。

管理端菜单由数据库表 `admin_menus` 驱动，支持 `icon` 图标编码和 `sort_order` 排序。默认结构包含 `系统管理`、`App 管理 / 用户`、`视频管理 / 视频列表 / 类别 / 转码历史` 和 `支付`。

## 启动移动端

```bash
cd front/mobile
flutter pub get
flutter run
```

移动端默认账号：

```text
user / 123456
```

## 认证与安全

- 管理端和移动端登录均返回 **签名 JWT**，请求时通过 `Authorization: Bearer <token>` 携带。管理端 token 12 小时过期，移动端 30 天。
- 密码使用 **bcrypt** 哈希存储；登录时按哈希比对（同时兼容尚未迁移的旧明文行，迁移 `012_hash_passwords.sql` 会把种子密码转为哈希）。
- 生产环境务必设置 `JWT_SECRET` 与 `HLS_SECRET`。
- 登录接口带**失败限流**（防爆破）：按客户端 IP 计数，管理端 5 分钟内失败 5 次、移动端 10 次后临时拒绝并返回 `429`（含 `Retry-After`），登录成功即清零。限流器在配置了可用 Redis 时使用**共享计数**（多实例集群级生效），Redis 不可用时自动**降级为进程内计数**，保证 Redis 故障也不影响登录。评论/评分发表复用同一限流器（按用户）。
- CORS 默认在 `local`/`dev` 放开（`allowed_origins: ["*"]`），生产环境应在 `config/prod.yml` 的 `allowed_origins` 或环境变量 `CORS_ALLOWED_ORIGINS`（逗号分隔）中配置白名单。

## 支付网关配置

通过环境变量提供密钥（**不要**把真实密钥写进 `config/*.yml` 或提交仓库）。未配置某渠道时，选择它下单会返回明确的「未配置」错误，本地可继续用 `mock`。

```text
# Stripe
STRIPE_SECRET_KEY / STRIPE_WEBHOOK_SECRET / STRIPE_SUCCESS_URL / STRIPE_CANCEL_URL
# PayPal
PAYPAL_CLIENT_ID / PAYPAL_CLIENT_SECRET / PAYPAL_WEBHOOK_ID / PAYPAL_BASE_URL
# 微信支付 v3（PEM 私钥可用 \n 转义或文件内容注入）
WECHAT_APP_ID / WECHAT_MCH_ID / WECHAT_SERIAL_NO / WECHAT_API_V3_KEY(32 字节) / WECHAT_PRIVATE_KEY / WECHAT_NOTIFY_URL
# 支付宝（RSA2）
ALIPAY_APP_ID / ALIPAY_PRIVATE_KEY / ALIPAY_PUBLIC_KEY / ALIPAY_GATEWAY / ALIPAY_NOTIFY_URL
```

回调地址默认 `<APP_PUBLIC_BASE_URL>/api/webhooks/{provider}`，微信/支付宝可用各自的 `*_NOTIFY_URL` 覆盖。

## 后端接口

认证相关：

```text
POST   /api/admin/login                 # 管理端登录，返回 JWT
POST   /api/mobile/login                # 移动端登录，返回 JWT
POST   /api/mobile/register             # 移动端自助注册，返回 JWT（带 IP 限流）
PUT    /api/mobile/password             # 移动端修改密码（需移动端 JWT，校验当前密码）
GET    /api/health                      # 健康检查（探测 DB/MinIO/Redis）
```

管理端 - 仪表盘（需管理员 JWT）：

```text
GET    /api/admin/stats                   # 视频/类别/用户/订单统计、已支付收入、热门视频
```

管理端 - 个人资料（需管理员 JWT）：

```text
GET    /api/admin/profile
PUT    /api/admin/profile/theme
POST   /api/admin/profile/avatar
GET    /api/admin/profile/assets/{avatar|thumbnail}
```

管理端 - RBAC（按按钮权限校验）：

```text
GET|POST          /api/admin/users
PUT|DELETE        /api/admin/users/{id}
GET|POST          /api/admin/roles
PUT|DELETE        /api/admin/roles/{id}
GET|POST          /api/admin/menus
PUT|DELETE        /api/admin/menus/{id}
```

管理端 - 业务管理（需管理员 JWT）：

```text
GET|POST          /api/admin/app-users
PUT|DELETE        /api/admin/app-users/{id}
GET|POST          /api/admin/products
PUT|DELETE        /api/admin/products/{id}
GET               /api/admin/orders               # 支持 ?status= 过滤、?format=csv 导出
DELETE            /api/admin/orders/{id}
POST              /api/admin/orders/{id}/refund   # 退款，需 payment:refund
GET|POST          /api/admin/videos
GET|PUT|DELETE    /api/admin/videos/{id}
POST              /api/admin/videos/{id}/upload
POST              /api/admin/videos/{id}/cover
POST              /api/admin/videos/{id}/ai-metadata
GET|POST          /api/admin/videos/{id}/transcode
GET               /api/admin/videos/{id}/tasks
DELETE            /api/admin/videos/{id}/tasks/{quality}
GET               /api/admin/video/transcode-tasks
DELETE            /api/admin/video/transcode-tasks/{id}
GET|POST          /api/admin/categories
PUT|DELETE        /api/admin/categories/{id}
GET|POST          /api/admin/series                # 剧集列表 / 新建（POST 需 series:create）
GET|PUT|DELETE    /api/admin/series/{id}           # 剧集详情（含分集）/ 编辑 / 删除（需 series:edit / series:delete）
POST              /api/admin/series/{id}/cover     # 上传剧集封面（series:edit）
GET|POST          /api/admin/series/{id}/episodes  # 分集列表 / 关联一个视频为分集（POST 需 series:edit）
DELETE            /api/admin/series/{id}/episodes/{videoId}  # 解除分集关联（series:edit）
GET               /api/admin/comments              # 评论审核列表，支持 q 搜索
DELETE            /api/admin/comments/{id}         # 删除评论，需 comment:delete
GET               /api/admin/audit-logs            # 操作审计日志，支持 q（管理员/路径）+ 分页
```

移动端（需移动端 JWT）：

```text
GET               /api/mobile/profile
GET|POST          /api/mobile/watch-history
GET|POST          /api/mobile/favorites
DELETE            /api/mobile/favorites/{videoId}
GET|PUT           /api/mobile/settings
```

App 公开 / 播放：

```text
GET    /api/categories
GET    /api/home                            # 首页推荐聚合：热门/最新/VIP 精选
GET    /api/series                          # 剧集列表（公开，排除已下架）
GET    /api/series/{id}                      # 剧集详情 + 已就绪分集列表（公开）
GET    /api/series/{id}/cover                # 剧集封面（公开）
GET    /api/products
POST   /api/orders
GET    /api/orders/{orderNo}
GET    /api/videos
GET    /api/videos/{id}
GET    /api/videos/{id}/play
GET    /api/videos/{id}/cover
POST   /api/videos/{id}/progress
GET    /api/videos/{id}/comments          # 评论列表 + 评分汇总（公开）
POST   /api/videos/{id}/comments          # 发表评论/评分（移动端 JWT）
DELETE /api/mobile/comments/{id}          # 删除自己的评论（移动端 JWT）
GET    /api/videos/{id}/danmaku           # 弹幕列表，按 time_ms 排序（公开；带 token 时附带 liked 标记）
POST   /api/videos/{id}/danmaku           # 发送弹幕（移动端 JWT，按用户限流）
DELETE /api/mobile/danmaku/{id}           # 删除自己的弹幕（移动端 JWT）
POST   /api/mobile/danmaku/{id}/like      # 点赞/取消点赞弹幕（移动端 JWT，幂等切换）
GET    /api/hls/{...}/master.m3u8
GET    /api/hls/{...}/index.m3u8
POST   /api/webhooks/{provider}           # 网关回调统一分发：stripe|paypal|wechat|alipay（各自验签后确认到账）
```

统一响应格式：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {}
}
```

## 常用命令

```bash
# PostgreSQL + MinIO
docker compose up -d postgres minio
docker compose logs -f postgres
docker compose logs -f minio

# 后端
cd backend
go run ./cmd/server
go test ./...

# 管理端
cd front/admin
npm run dev
npm run build

# 移动端
cd front/mobile
flutter run
```
