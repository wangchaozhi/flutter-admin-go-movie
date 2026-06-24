import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { BadgeCheck, Loader, Pencil, RefreshCw, Trash2 } from 'lucide-react'

import type {
  ApiResponse,
  CurrencyCode,
  Order,
  OrderStatus,
  Product,
  ProductKind,
  ProductStatus,
  Video,
} from '../../adminTypes'
import { PanelTitle } from '../../components/shared'

type ProductForm = {
  id?: number
  code: string
  name: string
  description: string
  kind: ProductKind
  price_cents: string
  currency: CurrencyCode
  duration_days: string
  video_id: string
  status: ProductStatus
}

const emptyProductForm: ProductForm = {
  code: '',
  name: '',
  description: '',
  kind: 'vip',
  price_cents: '',
  currency: 'USD',
  duration_days: '30',
  video_id: '',
  status: 'active',
}

const productStatusLabels: Record<ProductStatus, string> = {
  active: '上架',
  inactive: '下架',
}

const productKindLabels: Record<ProductKind, string> = {
  vip: '会员',
  video: '单片',
}

const orderStatusLabels: Record<OrderStatus, string> = {
  pending: '待支付',
  paying: '支付中',
  paid: '已支付',
  failed: '失败',
  cancelled: '已取消',
  refunded: '已退款',
}

const orderStatusClass: Record<OrderStatus, string> = {
  pending: 'status-uploaded',
  paying: 'status-transcoding',
  paid: 'status-ready',
  failed: 'status-failed',
  cancelled: 'status-offline',
  refunded: 'status-offline',
}

const currencyOptions: Array<{ value: CurrencyCode; label: string }> = [
  { value: 'CNY', label: 'CNY - 人民币' },
  { value: 'USD', label: 'USD - 美元' },
  { value: 'EUR', label: 'EUR - 欧元' },
  { value: 'JPY', label: 'JPY - 日元' },
  { value: 'HKD', label: 'HKD - 港币' },
  { value: 'TWD', label: 'TWD - 新台币' },
  { value: 'GBP', label: 'GBP - 英镑' },
  { value: 'AUD', label: 'AUD - 澳元' },
  { value: 'CAD', label: 'CAD - 加元' },
  { value: 'SGD', label: 'SGD - 新加坡元' },
]

const currencyValues = new Set<CurrencyCode>(currencyOptions.map((option) => option.value))

async function request<T>(url: string, token: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${token}`)
  if (init.body) headers.set('Content-Type', 'application/json')
  const res = await fetch(url, { ...init, headers })
  const body = (await res.json()) as ApiResponse<T>
  if (!res.ok || body.code !== 0) throw new Error(body.msg || '请求失败')
  return body.data as T
}

function money(order: { currency: string; amount_cents: number }) {
  return `${order.currency} ${(order.amount_cents / 100).toFixed(2)}`
}

function productMoney(product: Product) {
  return `${product.currency} ${(product.price_cents / 100).toFixed(2)}`
}

function formatDateTime(value?: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

function productToForm(product: Product): ProductForm {
  return {
    id: product.id,
    code: product.code,
    name: product.name,
    description: product.description,
    kind: product.kind,
    price_cents: String(product.price_cents),
    currency: currencyValues.has(product.currency) ? product.currency : 'USD',
    duration_days: String(product.duration_days),
    video_id: product.video_id ? String(product.video_id) : '',
    status: product.status,
  }
}

function statusBadge(status: OrderStatus) {
  return <span className={`status-badge ${orderStatusClass[status]}`}>{orderStatusLabels[status]}</span>
}

function orderUserLabel(order: Order) {
  return order.user?.username?.trim() || `#${order.user_id}`
}

function orderUserDetail(order: Order) {
  const nickname = order.user?.nickname?.trim()
  if (nickname) return `${nickname} · #${order.user_id}`
  if (order.user?.username) return `#${order.user_id}`
  return ''
}

export function PaymentManagementSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [products, setProducts] = useState<Product[]>([])
  const [orders, setOrders] = useState<Order[]>([])
  const [videos, setVideos] = useState<Video[]>([])
  const [productForm, setProductForm] = useState<ProductForm>(emptyProductForm)
  const [videoKeyword, setVideoKeyword] = useState('')
  const [videoPickerOpen, setVideoPickerOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [savingProduct, setSavingProduct] = useState(false)
  const [error, setError] = useState('')

  const productNameByID = useMemo(
    () => new Map(products.map((product) => [product.id, product.name])),
    [products],
  )
  const videoTitleByID = useMemo(
    () => new Map(videos.map((video) => [video.id, video.title])),
    [videos],
  )
  const canManageProducts = can('payment:product')
  const canManageOrders = can('payment:order')
  const filteredVideos = useMemo(() => {
    const keyword = videoKeyword.trim().toLowerCase()
    const list = keyword
      ? videos.filter((video) =>
          String(video.id).includes(keyword) ||
          video.title.toLowerCase().includes(keyword),
        )
      : videos
    return list.slice(0, 80)
  }, [videoKeyword, videos])

  async function loadPayments() {
    setLoading(true)
    setError('')
    try {
      const [nextProducts, nextOrders, nextVideos] = await Promise.all([
        request<Product[]>('/api/admin/products', token),
        request<Order[]>('/api/admin/orders', token),
        request<Video[]>('/api/admin/videos', token),
      ])
      setProducts(nextProducts ?? [])
      setOrders(nextOrders ?? [])
      setVideos(nextVideos ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载支付数据失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadPayments()
  }, [token])

  async function handleSaveProduct(event: FormEvent) {
    event.preventDefault()
    if (!canManageProducts) return
    if (productForm.kind === 'video' && !productForm.video_id) {
      setError('请选择影片')
      return
    }
    setSavingProduct(true)
    setError('')
    try {
      const body = {
        code: productForm.code.trim(),
        name: productForm.name.trim(),
        description: productForm.description.trim(),
        kind: productForm.kind,
        price_cents: Number(productForm.price_cents),
        currency: productForm.currency,
        duration_days: Number(productForm.duration_days || 0),
        video_id: productForm.kind === 'video' && productForm.video_id ? Number(productForm.video_id) : null,
        status: productForm.status,
      }
      const url = productForm.id ? `/api/admin/products/${productForm.id}` : '/api/admin/products'
      const method = productForm.id ? 'PUT' : 'POST'
      await request<Product>(url, token, { method, body: JSON.stringify(body) })
      resetProductForm()
      await loadPayments()
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存套餐失败')
    } finally {
      setSavingProduct(false)
    }
  }

  async function handleDeleteProduct(product: Product) {
    if (!canManageProducts) return
    if (!window.confirm(`确认删除套餐「${product.name}」？`)) return
    setError('')
    try {
      await request<unknown>(`/api/admin/products/${product.id}`, token, { method: 'DELETE' })
      if (productForm.id === product.id) resetProductForm()
      await loadPayments()
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除套餐失败')
    }
  }

  function editProduct(product: Product) {
    const nextForm = productToForm(product)
    setProductForm(nextForm)
    const selected = videos.find((video) => String(video.id) === nextForm.video_id)
    setVideoKeyword(selected?.title ?? nextForm.video_id)
    setVideoPickerOpen(false)
  }

  function resetProductForm() {
    setProductForm(emptyProductForm)
    setVideoKeyword('')
    setVideoPickerOpen(false)
  }

  function selectVideo(video: Video) {
    setProductForm({ ...productForm, video_id: String(video.id) })
    setVideoKeyword(video.title)
    setVideoPickerOpen(false)
  }

  async function handleDeleteOrder(order: Order) {
    if (!canManageOrders) return
    if (!window.confirm(`确认删除订单「${order.order_no}」？`)) return
    setError('')
    try {
      await request<unknown>(`/api/admin/orders/${order.id}`, token, { method: 'DELETE' })
      await loadPayments()
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除订单失败')
    }
  }

  return (
    <div className="stack">
      <section className="panel">
        <div className="section-header">
          <PanelTitle title="支付概览" />
          <button className="ghost-button" type="button" onClick={loadPayments} disabled={loading}>
            <RefreshCw size={15} className={loading ? 'spin' : undefined} />
            刷新
          </button>
        </div>
        {error && <span className="status error">{error}</span>}
        <div className="summary-grid">
          <div className="summary-card">
            <small>套餐</small>
            <strong>{products.length}</strong>
          </div>
          <div className="summary-card">
            <small>订单</small>
            <strong>{orders.length}</strong>
          </div>
          <div className="summary-card">
            <small>已支付</small>
            <strong>{orders.filter((order) => order.status === 'paid').length}</strong>
          </div>
        </div>
      </section>

      <section className="content-grid">
        <section className="table-panel">
          <PanelTitle title="套餐" count={products.length} />
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>编码</th>
                  <th>名称</th>
                  <th>类型</th>
                  <th>价格</th>
                  <th>有效期</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {products.map((product) => (
                  <tr key={product.id} className={productForm.id === product.id ? 'row-active' : ''}>
                    <td>
                      <strong>{product.code}</strong>
                      {product.video_id && (
                        <small>#{product.video_id} {videoTitleByID.get(product.video_id) ?? '未知影片'}</small>
                      )}
                    </td>
                    <td>
                      {product.name}
                      {product.description && <small>{product.description}</small>}
                    </td>
                    <td>{productKindLabels[product.kind]}</td>
                    <td>{productMoney(product)}</td>
                    <td>{product.kind === 'vip' ? `${product.duration_days} 天` : '-'}</td>
                    <td>
                      <span className={`status-badge ${product.status === 'active' ? 'status-ready' : 'status-offline'}`}>
                        {productStatusLabels[product.status]}
                      </span>
                    </td>
                    <td>
                      <div className="row-actions">
                        {canManageProducts && (
                          <button type="button" onClick={() => editProduct(product)}>
                            <Pencil size={13} />
                            编辑
                          </button>
                        )}
                        {canManageProducts && (
                          <button className="danger" type="button" onClick={() => void handleDeleteProduct(product)}>
                            <Trash2 size={13} />
                            删除
                          </button>
                        )}
                        {!canManageProducts && <span className="muted-action">无权限</span>}
                      </div>
                    </td>
                  </tr>
                ))}
                {products.length === 0 && (
                  <tr>
                    <td colSpan={7} className="empty-cell">暂无套餐</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>

        <form className="editor-panel" onSubmit={handleSaveProduct}>
          <PanelTitle title={productForm.id ? `编辑套餐 #${productForm.id}` : '新增套餐'} />
          <label>
            编码
            <input
              value={productForm.code}
              onChange={(event) => setProductForm({ ...productForm, code: event.target.value })}
              placeholder="vip_monthly"
              required
            />
          </label>
          <label>
            名称
            <input
              value={productForm.name}
              onChange={(event) => setProductForm({ ...productForm, name: event.target.value })}
              placeholder="VIP 月卡"
              required
            />
          </label>
          <label>
            描述
            <input
              value={productForm.description}
              onChange={(event) => setProductForm({ ...productForm, description: event.target.value })}
              placeholder="30 天会员权益"
            />
          </label>
          <div className="form-split">
            <label>
              类型
              <select
                value={productForm.kind}
                onChange={(event) => {
                  const kind = event.target.value as ProductKind
                  setProductForm({
                    ...productForm,
                    kind,
                    video_id: kind === 'video' ? productForm.video_id : '',
                    duration_days: kind === 'video' ? '0' : productForm.duration_days || '30',
                  })
                  if (kind !== 'video') {
                    setVideoKeyword('')
                    setVideoPickerOpen(false)
                  }
                }}
              >
                <option value="vip">会员</option>
                <option value="video">单片</option>
              </select>
            </label>
            <label>
              状态
              <select
                value={productForm.status}
                onChange={(event) => setProductForm({ ...productForm, status: event.target.value as ProductStatus })}
              >
                <option value="active">上架</option>
                <option value="inactive">下架</option>
              </select>
            </label>
          </div>
          <div className="form-split">
            <label>
              价格（分）
              <input
                type="number"
                min="1"
                value={productForm.price_cents}
                onChange={(event) => setProductForm({ ...productForm, price_cents: event.target.value })}
                placeholder="999"
                required
              />
            </label>
            <label>
              币种
              <select
                value={productForm.currency}
                onChange={(event) => setProductForm({ ...productForm, currency: event.target.value as CurrencyCode })}
              >
                {currencyOptions.map((option) => (
                  <option key={option.value} value={option.value}>{option.label}</option>
                ))}
              </select>
            </label>
          </div>
          {productForm.kind === 'vip' && (
            <label>
              有效期（天）
              <input
                type="number"
                min="1"
                value={productForm.duration_days}
                onChange={(event) => setProductForm({ ...productForm, duration_days: event.target.value })}
                required
              />
            </label>
          )}
          {productForm.kind === 'video' && (
            <label>
              影片
              <div
                className="searchable-select"
                onBlur={(event) => {
                  if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                    setVideoPickerOpen(false)
                  }
                }}
              >
                <input
                  value={videoKeyword}
                  onChange={(event) => {
                    setVideoKeyword(event.target.value)
                    setVideoPickerOpen(true)
                  }}
                  onFocus={() => setVideoPickerOpen(true)}
                  placeholder="搜索影片名称或 ID"
                />
                {videoPickerOpen && (
                  <div className="searchable-options" role="listbox" aria-label="影片列表">
                    {filteredVideos.map((video) => (
                      <button
                        className={productForm.video_id === String(video.id) ? 'active' : ''}
                        key={video.id}
                        type="button"
                        onClick={() => selectVideo(video)}
                      >
                        <span>#{video.id}</span>
                        <strong>{video.title}</strong>
                      </button>
                    ))}
                    {filteredVideos.length === 0 && (
                      <span className="searchable-empty">没有匹配影片</span>
                    )}
                  </div>
                )}
              </div>
            </label>
          )}
          <div className="form-actions">
            {canManageProducts && (
              <button className="primary-button" type="submit" disabled={savingProduct}>
                {savingProduct ? <Loader size={14} className="spin" /> : <BadgeCheck size={14} />}
                {productForm.id ? '保存' : '新增'}
              </button>
            )}
            <button className="ghost-button" type="button" onClick={resetProductForm}>
              重置
            </button>
          </div>
        </form>
      </section>

      <section className="table-panel">
        <PanelTitle title="订单" count={orders.length} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>订单号</th>
                <th>用户</th>
                <th>套餐</th>
                <th>渠道</th>
                <th>金额</th>
                <th>状态</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr key={order.id}>
                  <td>
                    <strong>{order.order_no}</strong>
                    {order.provider_payment_id && <small>{order.provider_payment_id}</small>}
                  </td>
                  <td>
                    <strong>{orderUserLabel(order)}</strong>
                    {orderUserDetail(order) && <small>{orderUserDetail(order)}</small>}
                  </td>
                  <td>{order.product?.name ?? productNameByID.get(order.product_id) ?? '-'}</td>
                  <td>{order.provider}</td>
                  <td>{money(order)}</td>
                  <td>{statusBadge(order.status)}</td>
                  <td>{formatDateTime(order.created_at)}</td>
                  <td>
                    <div className="row-actions">
                      {canManageOrders ? (
                        <button className="danger" type="button" onClick={() => void handleDeleteOrder(order)}>
                          <Trash2 size={13} />
                          删除
                        </button>
                      ) : (
                        <span className="muted-action">无权限</span>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
              {orders.length === 0 && (
                <tr>
                  <td colSpan={8} className="empty-cell">暂无订单</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
