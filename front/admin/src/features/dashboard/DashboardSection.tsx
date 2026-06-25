import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'

import type { ApiResponse, DashboardStats } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'

const videoStatusLabels: Record<string, string> = {
  uploading: '上传中',
  extracting: '提取中',
  uploaded: '已上传',
  transcoding: '转码中',
  ready: '已就绪',
  failed: '失败',
  offline: '已下线',
}

const orderStatusLabels: Record<string, string> = {
  pending: '待支付',
  paying: '支付中',
  paid: '已支付',
  failed: '失败',
  cancelled: '已取消',
  refunded: '已退款',
}

function formatMoney(currency: string, cents: number) {
  return `${currency} ${(cents / 100).toFixed(2)}`
}

export function DashboardSection({ token }: { token: string }) {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const res = await fetch('/api/admin/stats', { headers: { Authorization: `Bearer ${token}` } })
      const body = (await res.json()) as ApiResponse<DashboardStats>
      if (!res.ok || body.code !== 0) throw new Error(body.msg || '加载失败')
      setStats(body.data ?? null)
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载统计失败')
    } finally {
      setLoading(false)
    }
  }

  // load on mount only
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [])

  return (
    <div className="stack">
      <section className="panel">
        <div className="section-header">
          <PanelTitle title="数据概览" />
          <button className="ghost-button" type="button" onClick={load} disabled={loading}>
            <RefreshCw size={15} className={loading ? 'spin' : undefined} />
            刷新
          </button>
        </div>
        {error && <span className="status error">{error}</span>}

        {stats && (
          <>
            <div className="summary-grid">
              <div className="summary-card">
                <small>视频总数</small>
                <strong>{stats.videos.total}</strong>
              </div>
              <div className="summary-card">
                <small>VIP 专属 / 免费</small>
                <strong>{stats.videos.vip} / {stats.videos.free}</strong>
              </div>
              <div className="summary-card">
                <small>类别</small>
                <strong>{stats.categories.total}</strong>
              </div>
              <div className="summary-card">
                <small>App 用户</small>
                <strong>{stats.users.total}</strong>
              </div>
              <div className="summary-card">
                <small>VIP 会员 / 已封禁</small>
                <strong>{stats.users.vip} / {stats.users.banned}</strong>
              </div>
              <div className="summary-card">
                <small>订单总数</small>
                <strong>{stats.orders.total}</strong>
              </div>
            </div>

            <div className="dashboard-columns">
              <section className="panel-inset">
                <PanelTitle title="收入（已支付）" />
                {stats.revenue.length === 0 ? (
                  <p className="muted">暂无已支付订单</p>
                ) : (
                  <ul className="stat-list">
                    {stats.revenue.map((row) => (
                      <li key={row.currency}>
                        <span>{row.currency}</span>
                        <strong>{formatMoney(row.currency, row.amount_cents)}</strong>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="panel-inset">
                <PanelTitle title="订单状态" />
                {stats.orders.by_status.length === 0 ? (
                  <p className="muted">暂无订单</p>
                ) : (
                  <ul className="stat-list">
                    {stats.orders.by_status.map((row) => (
                      <li key={row.key}>
                        <span>{orderStatusLabels[row.key] ?? row.key}</span>
                        <strong>{row.count}</strong>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="panel-inset">
                <PanelTitle title="视频状态" />
                {stats.videos.by_status.length === 0 ? (
                  <p className="muted">暂无视频</p>
                ) : (
                  <ul className="stat-list">
                    {stats.videos.by_status.map((row) => (
                      <li key={row.key}>
                        <span>{videoStatusLabels[row.key] ?? row.key}</span>
                        <strong>{row.count}</strong>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="panel-inset">
                <PanelTitle title="热门视频（播放量）" />
                {stats.top_videos.length === 0 ? (
                  <p className="muted">暂无播放记录</p>
                ) : (
                  <ul className="stat-list">
                    {stats.top_videos.map((row) => (
                      <li key={row.video_id}>
                        <span title={row.title}>{row.title || `#${row.video_id}`}</span>
                        <strong>{row.plays}</strong>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </div>
          </>
        )}
      </section>
    </div>
  )
}
