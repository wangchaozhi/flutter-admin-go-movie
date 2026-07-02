import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'

import type { DashboardStats } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'
import { adminRequest } from '../../core/adminApi'
import { showError } from '../../core/feedback'

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

// RevenueTrendChart draws a lightweight, dependency-free SVG bar chart of daily
// paid revenue for the primary currency over the last 30 days.
function RevenueTrendChart({ trend }: { trend: DashboardStats['revenue_trend'] }) {
  const points = trend.points ?? []
  const totalCents = points.reduce((sum, p) => sum + p.amount_cents, 0)
  if (!trend.currency || totalCents === 0) {
    return <p className="muted">暂无收入数据</p>
  }

  const width = 720
  const height = 160
  const padX = 8
  const padY = 16
  const maxCents = Math.max(1, ...points.map((p) => p.amount_cents))
  const slot = (width - padX * 2) / points.length
  const barWidth = Math.max(2, slot - 3)

  return (
    <div className="revenue-trend">
      <div className="revenue-trend-head">
        <span>近 30 天 · {trend.currency}</span>
        <strong>{formatMoney(trend.currency, totalCents)}</strong>
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="revenue-trend-chart" preserveAspectRatio="none" role="img" aria-label="收入趋势">
        {points.map((p, i) => {
          const barHeight = (p.amount_cents / maxCents) * (height - padY * 2)
          const x = padX + i * slot
          const y = height - padY - barHeight
          return (
            <rect
              key={p.date}
              x={x}
              y={y}
              width={barWidth}
              height={Math.max(barHeight, p.amount_cents > 0 ? 1 : 0)}
              rx={1.5}
              fill="var(--accent, #6366f1)"
            >
              <title>{`${p.date}\n${formatMoney(trend.currency, p.amount_cents)} · ${p.orders} 单`}</title>
            </rect>
          )
        })}
      </svg>
    </div>
  )
}

export function DashboardSection({ token }: { token: string }) {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await adminRequest<DashboardStats>('/api/admin/stats', { token })
      setStats(data ?? null)
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载统计失败'
      setError(message)
      showError(message)
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

            <section className="panel-inset">
              <PanelTitle title="收入趋势" />
              <RevenueTrendChart trend={stats.revenue_trend} />
            </section>

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
