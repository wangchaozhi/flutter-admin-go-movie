# 移动端“我的”模块技术方案

## 目标

将移动端底部导航的“我的”从占位 Tab 升级为个人中心，承载用户资料、VIP 会员、观看记录、收藏/片单、订单、设置和退出登录等入口。第一版优先复用现有视频、VIP 和支付能力，补齐必要的移动端资料与订单读取接口。

## 页面结构

“我的”Tab 使用单页聚合布局：

1. 顶部用户区
   - 展示头像、用户名、账号状态。
   - 展示 VIP 徽章与会员状态。
   - 非 VIP 用户显示“开通 VIP”按钮，VIP 用户显示到期时间。

2. VIP 会员卡片
   - 显示会员权益摘要。
   - 点击进入现有 `/vip` 页面购买或续费。

3. 快捷功能入口
   - 观看记录：第一版展示入口和最近观看摘要，后续接入列表接口。
   - 我的收藏：第一版展示入口，后续新增收藏表与接口。
   - 订单记录：第一版接入 `/api/orders` 的 GET 列表。
   - 设置：第一版提供清理缓存、关于 App、退出登录等基础入口。

4. 订单预览
   - 展示最近 3 条订单。
   - 显示订单号、商品名、支付状态、金额。
   - 空状态显示“暂无订单”。

5. 退出登录
   - 放在页面底部或设置入口中，使用危险色降低误触风险。

## 数据与接口

### 已有能力

- `POST /api/mobile/login`：移动端登录。
- `GET /api/products`：获取 VIP 商品。
- `POST /api/orders`：创建订单。
- `GET /api/orders/{order_no}`：读取单个订单。
- `GET /api/videos`：视频列表。
- `GET/POST /api/videos/{id}/progress`：单个视频播放进度。

### 第一版新增接口

1. `GET /api/mobile/profile`
   - 需要移动端 Bearer Token。
   - 返回当前用户 `id`、`username`、`nickname`、`email`、`status`、`vip_until`、`is_vip`。

2. `GET /api/orders`
   - 保留现有 `POST /api/orders` 创建订单。
   - 新增 GET 分支，返回当前移动端用户最近订单列表。
   - 支持 `limit` 参数，默认 20，最大 50。
   - 预加载 `Product`，方便移动端展示商品名称。

### 后续接口

- `GET /api/mobile/watch-history`：观看记录列表，关联视频信息。
- `POST/DELETE /api/mobile/favorites/{video_id}`：收藏/取消收藏。
- `GET /api/mobile/favorites`：收藏列表。
- `GET /api/mobile/settings` 与 `PUT /api/mobile/settings`：用户偏好。

## Flutter 实现

### 文件

- `front/mobile/lib/features/home/mobile_home_page.dart`
  - 将 `_selectedNav == 3` 时的内容切换为“我的”页面。
  - 复用现有 `/vip` 路由和退出登录逻辑。
  - 第一版在同文件内实现私有 Widget，保持改动集中。

### 状态

- 首页视频列表仍由首页 Tab 使用。
- “我的”Tab 独立加载 profile 与 orders。
- 加载失败时保留页面骨架，并展示轻量错误文案。

### 交互

- 底部导航切到“我的”时展示个人中心。
- 顶部右上角头像菜单继续保留，作为快捷入口。
- VIP 卡片点击跳转 `/vip`。
- 订单记录卡片展示最近订单；第一版不做订单详情页。

## 风险与边界

- 观看记录、收藏、下载目前缺少完整列表数据结构，第一版以入口和空状态为主。
- VIP 到期时间由后端 `vip_until` 判断，客户端只负责展示。
- 订单接口需要移动端 Token 过滤，禁止返回其他用户订单。
