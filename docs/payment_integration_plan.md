# Stripe + PayPal 三端支付接入技术方案

## 目标

为当前 Go 后端、React 管理端、Flutter 移动端接入 Stripe 与 PayPal，支撑影片/VIP 权益购买、订单管理、支付回调确认、移动端支付跳转与结果查询。

本方案以服务端为可信源：移动端只发起订单和跳转支付，支付成功必须由 Stripe/PayPal Webhook 或服务端主动确认后才授予权益。

## 官方接口口径

- Stripe：使用 Checkout Session 或 PaymentIntent 创建支付，Webhook 处理 `checkout.session.completed`、`payment_intent.succeeded`、`payment_intent.payment_failed`。Stripe 官方要求 Webhook 使用原始请求体与 `Stripe-Signature` 验签。
- PayPal：使用 Orders v2 创建订单，用户批准后服务端调用 capture，Webhook 处理 `CHECKOUT.ORDER.APPROVED`、`PAYMENT.CAPTURE.COMPLETED`、`PAYMENT.CAPTURE.DENIED` 等事件。Webhook 通过 PayPal Verify Webhook Signature API 或证书签名链验证。

参考：

- Stripe Checkout Sessions: https://docs.stripe.com/api/checkout/sessions
- Stripe Webhooks: https://docs.stripe.com/webhooks
- PayPal Orders v2: https://developer.paypal.com/docs/api/orders/v2/
- PayPal Webhooks v1: https://developer.paypal.com/docs/api/webhooks/v1/

## 三端职责

### 后端

- 提供商品/套餐、订单、支付会话、支付状态查询接口。
- 生成 Stripe Checkout Session 与 PayPal Order。
- 接收并验签 Stripe/PayPal Webhook，幂等更新支付状态。
- 在支付成功后授予用户权益，例如 VIP 到期时间或影片购买记录。
- 为管理端提供订单、支付流水、退款/重试入口。
- 保留 mock provider，方便本地无密钥开发。

### Flutter 移动端

- 展示可购买套餐/影片权益。
- 调用后端创建订单与支付会话。
- 通过浏览器或 WebView 打开 Stripe Checkout / PayPal approve URL。
- 支付完成跳回 App 后轮询订单状态。
- 根据权益状态控制 VIP 影片播放。

### React 管理端

- 查看订单、支付状态、provider、金额、用户、创建/支付时间。
- 配置套餐上下架与价格。
- 人工标记异常订单、触发退款或重新同步支付状态。
- 查看 Webhook 事件与失败原因，方便排障。

## 数据模型

### products

套餐或可售商品。

- `id`
- `code`：业务唯一编码，例如 `vip_monthly`
- `name`
- `description`
- `kind`：`vip`、`video`
- `price_cents`
- `currency`
- `duration_days`：VIP 商品有效期
- `video_id`：单片购买时关联
- `status`：`active`、`inactive`
- `created_at`、`updated_at`

### orders

业务订单。

- `id`
- `order_no`：对外订单号
- `user_id`
- `product_id`
- `provider`：`stripe`、`paypal`、`mock`
- `status`：`pending`、`paying`、`paid`、`failed`、`cancelled`、`refunded`
- `amount_cents`
- `currency`
- `provider_order_id`
- `provider_payment_id`
- `checkout_url`
- `paid_at`
- `expires_at`
- `created_at`、`updated_at`

### payment_events

Webhook 与主动同步事件，保证幂等和可审计。

- `id`
- `provider`
- `event_id`
- `event_type`
- `order_no`
- `payload`
- `processed_at`
- `created_at`

### mobile_users 扩展

- `vip_until`：VIP 权益到期时间。

## 状态机

- `pending`：订单已创建，未创建支付会话。
- `paying`：已创建第三方支付会话，等待用户支付。
- `paid`：支付成功并已授予权益。
- `failed`：支付失败或 provider 明确拒绝。
- `cancelled`：用户取消或订单超时。
- `refunded`：退款成功。

状态更新只允许向前推进。Webhook 需要按 `provider + event_id` 幂等去重；重复事件返回成功但不重复发权益。

## API 设计

### 移动端

- `GET /api/products`：可购买商品列表。
- `POST /api/orders`：创建订单并返回支付跳转地址。
- `GET /api/orders/{order_no}`：查询订单状态。
- `POST /api/orders/{order_no}/cancel`：取消未支付订单。

创建订单请求：

```json
{
  "product_code": "vip_monthly",
  "provider": "stripe"
}
```

响应：

```json
{
  "order_no": "ORD202605120001",
  "status": "paying",
  "provider": "stripe",
  "checkout_url": "https://checkout.stripe.com/..."
}
```

### Webhook

- `POST /api/webhooks/stripe`
- `POST /api/webhooks/paypal`

Webhook 路由不能提前 JSON decode 后再验签。Stripe 必须读取原始 body；PayPal 需保留原始 payload 并带上 webhook headers 做签名校验。

### 管理端

- `GET /api/admin/products`
- `POST /api/admin/products`
- `PUT /api/admin/products/{id}`
- `GET /api/admin/orders`
- `GET /api/admin/orders/{order_no}`
- `POST /api/admin/orders/{order_no}/sync`
- `POST /api/admin/orders/{order_no}/refund`

## 配置

后端环境变量：

- `PAYMENT_MOCK=true|false`
- `APP_PUBLIC_BASE_URL`
- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- `STRIPE_SUCCESS_URL`
- `STRIPE_CANCEL_URL`
- `PAYPAL_CLIENT_ID`
- `PAYPAL_CLIENT_SECRET`
- `PAYPAL_WEBHOOK_ID`
- `PAYPAL_BASE_URL=https://api-m.sandbox.paypal.com`
- `PAYMENT_DEFAULT_CURRENCY=USD`

移动端：

- `API_BASE_URL`
- Deep link scheme，例如 `movieapp://payment/result`

## 安全要求

- 第三方密钥只放后端环境变量，不进入移动端或管理端。
- 订单金额以数据库商品价格为准，前端不得传金额。
- Webhook 必须验签并幂等。
- 支付成功授予权益必须在数据库事务内完成。
- 退款与人工调整需要管理员鉴权，并写审计事件。
- 所有支付状态查询按当前登录用户过滤，禁止越权读取他人订单。

## 分步执行

1. 后端基础：新增表、模型、配置、订单状态机、mock provider。
2. Stripe：创建 Checkout Session、接收 Webhook、支付成功发放权益。
3. PayPal：创建 Orders v2、capture、接收 Webhook、支付成功发放权益。
4. 移动端：套餐页、订单创建、外部浏览器跳转、结果轮询。
5. 管理端：商品维护、订单列表、支付事件查看。
6. 联调：Stripe test mode、PayPal sandbox、Webhook CLI/公网回调测试。
7. 上线：密钥轮换、Webhook endpoint 配置、监控告警、退款流程验证。

## 当前第一阶段交付范围

本次先交付：

- 数据库迁移与 Go 模型。
- `/api/products`、`/api/orders`、`/api/orders/{order_no}`。
- mock provider 支付流。
- Stripe/PayPal provider 接口骨架与配置校验。
- Webhook 路由占位与原始 body 处理框架。

Stripe 和 PayPal 真实支付请求会在后续阶段接入官方 SDK/API，并用 sandbox/test mode 验证。
