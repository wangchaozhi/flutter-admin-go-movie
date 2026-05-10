# flutter-admin-go

一个包含 Go 后端、React 管理端和 Flutter 移动端的全栈示例项目。后端提供登录、用户、角色、菜单和按钮权限接口，使用 PostgreSQL 存储数据，数据访问层基于 GORM。

## 项目结构

```text
.
├── docker-compose.yml                 # PostgreSQL + MinIO 本地开发环境
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

首次启动会自动执行 `backend/internal/store/migrations/*.sql`。已执行版本记录在 `schema_migrations` 表中。
后端也会自动创建头像 bucket。用户主题、头像对象 key 和缩略图对象 key 由迁移文件写入 `admin_users` 扩展字段。

健康检查：

```text
GET /api/health
```

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

`admin` 拥有全部菜单和按钮权限。按钮权限包括：

```text
user:create
user:edit
user:delete
role:create
role:edit
role:delete
menu:create
menu:edit
menu:delete
```

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

## 后端接口

```text
POST   /api/admin/login
GET    /api/admin/profile
PUT    /api/admin/profile/theme
POST   /api/admin/profile/avatar
GET    /api/admin/profile/assets/avatar
GET    /api/admin/profile/assets/thumbnail
POST   /api/mobile/login
GET    /api/admin/users
POST   /api/admin/users
PUT    /api/admin/users/{id}
DELETE /api/admin/users/{id}
GET    /api/admin/roles
POST   /api/admin/roles
PUT    /api/admin/roles/{id}
DELETE /api/admin/roles/{id}
GET    /api/admin/menus
POST   /api/admin/menus
PUT    /api/admin/menus/{id}
DELETE /api/admin/menus/{id}
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
