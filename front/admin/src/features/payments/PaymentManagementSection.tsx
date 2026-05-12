import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'

import type { ApiResponse, Order, Product } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'

async function request<T>(url: string, token: string): Promise<T> {
  const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
  const body = (await res.json()) as ApiResponse<T>
  if (!res.ok || body.code !== 0) throw new Error(body.msg || '请求失败')
  return body.data as T
}

export function PaymentManagementSection({ token }: { token: string; can: (permission: string) => boolean }) {
  const [products, setProducts] = useState<Product[]>([])
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function loadPayments() {
    setLoading(true)
    setError('')
    try {
      const [nextProducts, nextOrders] = await Promise.all([
        request<Product[]>('/api/admin/products', token),
        request<Order[]>('/api/admin/orders', token),
      ])
      setProducts(nextProducts ?? [])
      setOrders(nextOrders ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载支付数据失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadPayments()
  }, [token])

  return (
    <div className="stack">
      <section className="panel">
        <div className="section-header">
          <PanelTitle title="支付概览" />
          <button className="ghost-button" type="button" onClick={loadPayments} disabled={loading}>
            <RefreshCw size={15} />
            刷新
          </button>
        </div>
        {error && <span className="status error">{error}</span>}
        <div className="summary-grid">
          <div className="summary-card">
            <small>商品</small>
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

      <section className="panel">
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
              </tr>
            </thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id}>
                  <td>{product.code}</td>
                  <td>{product.name}</td>
                  <td>{product.kind}</td>
                  <td>{product.currency} {(product.price_cents / 100).toFixed(2)}</td>
                  <td>{product.duration_days} 天</td>
                  <td><span className={product.status === 'active' ? 'status ready' : 'status'}>{product.status}</span></td>
                </tr>
              ))}
              {products.length === 0 && <tr><td colSpan={6}>暂无套餐</td></tr>}
            </tbody>
          </table>
        </div>
      </section>

      <section className="panel">
        <PanelTitle title="订单" count={orders.length} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>订单号</th>
                <th>用户</th>
                <th>商品</th>
                <th>渠道</th>
                <th>金额</th>
                <th>状态</th>
                <th>创建时间</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr key={order.id}>
                  <td>{order.order_no}</td>
                  <td>{order.user_id}</td>
                  <td>{order.product?.name ?? '-'}</td>
                  <td>{order.provider}</td>
                  <td>{order.currency} {(order.amount_cents / 100).toFixed(2)}</td>
                  <td><span className={order.status === 'paid' ? 'status ready' : 'status'}>{order.status}</span></td>
                  <td>{new Date(order.created_at).toLocaleString()}</td>
                </tr>
              ))}
              {orders.length === 0 && <tr><td colSpan={7}>暂无订单</td></tr>}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
