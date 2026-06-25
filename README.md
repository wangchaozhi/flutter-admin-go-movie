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
- 内容管理：视频、类别、影片资料（演员、导演、地区、年份、语言、类型）、转码任务、转码历史记录和多清晰度 HLS 资源管理。
- AI 元信息补全：管理端可调用 DeepSeek / OpenAI-compatible API 自动生成视频简介、看点和标签；移动端播放页展示简介、看点、标签和结构化影片资料。
- App 管理：移动端用户资料、状态和登录密码维护。
- 支付管理：会员套餐、单片套餐、订单列表和订单删除，订单列表会展示 App 用户名。
- 移动端：视频浏览、播放、收藏、观看历史、个人设置、商品和订单。

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

健康检查：

```text
GET /api/health
```

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
- 登录接口带**失败限流**（防爆破）：按客户端 IP 计数，管理端 5 分钟内失败 5 次、移动端 10 次后临时拒绝并返回 `429`（含 `Retry-After`），登录成功即清零。限流为进程内实现，多实例部署应改用 Redis 等共享存储统一计数。
- CORS 默认在 `local`/`dev` 放开（`allowed_origins: ["*"]`），生产环境应在 `config/prod.yml` 的 `allowed_origins` 或环境变量 `CORS_ALLOWED_ORIGINS`（逗号分隔）中配置白名单。

## 后端接口

认证相关：

```text
POST   /api/admin/login                 # 管理端登录，返回 JWT
POST   /api/mobile/login                # 移动端登录，返回 JWT
GET    /api/health                      # 健康检查
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
GET               /api/admin/orders
DELETE            /api/admin/orders/{id}
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
GET    /api/products
POST   /api/orders
GET    /api/orders/{orderNo}
GET    /api/videos
GET    /api/videos/{id}
GET    /api/videos/{id}/play
GET    /api/videos/{id}/cover
POST   /api/videos/{id}/progress
GET    /api/hls/{...}/master.m3u8
GET    /api/hls/{...}/index.m3u8
POST   /api/webhooks/stripe
POST   /api/webhooks/paypal
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
