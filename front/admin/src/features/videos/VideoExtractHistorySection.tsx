import { useCallback, useEffect, useState } from 'react'
import {
  AudioLines,
  CheckCircle2,
  Clock3,
  Captions,
  Loader,
  RefreshCw,
  Search,
  Trash2,
  XCircle,
} from 'lucide-react'

import type { ExtractHistoryItem, ExtractTaskStatus, Paged } from '../../adminTypes'
import { PanelTitle, Pagination } from '../../components/shared'
import { adminRequest } from '../../core/adminApi'
import { confirmAction, showError, showSuccess } from '../../core/feedback'

const PER_PAGE = 20

const STATUS_LABEL: Record<ExtractTaskStatus, string> = {
  processing: '提取中',
  success: '成功',
  failed: '失败',
  canceled: '已取消',
}

const STATUS_CLASS: Record<string, string> = {
  processing: 'status-transcoding',
  success: 'status-ready',
  failed: 'status-failed',
  canceled: 'status-offline',
}

const statusOptions: Array<{ value: 'all' | ExtractTaskStatus; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'processing', label: '提取中' },
  { value: 'success', label: '成功' },
  { value: 'failed', label: '失败' },
  { value: 'canceled', label: '已取消' },
]

function isActiveStatus(status: string) {
  return status === 'processing'
}

function formatDateTime(value?: string | null) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('zh-CN', { hour12: false })
}

function formatElapsed(task: Pick<ExtractHistoryItem, 'started_at' | 'finished_at' | 'status'>) {
  if (!task.started_at) return '—'
  const start = new Date(task.started_at).getTime()
  if (Number.isNaN(start)) return '—'
  const end = task.finished_at
    ? new Date(task.finished_at).getTime()
    : task.status === 'processing'
      ? Date.now()
      : Number.NaN
  if (Number.isNaN(end) || end < start) return '—'
  const sec = Math.round((end - start) / 1000)
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) return `${h}时${m}分${s}秒`
  if (m > 0) return `${m}分${s}秒`
  return `${s}秒`
}

export function VideoExtractHistorySection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [tasks, setTasks] = useState<ExtractHistoryItem[]>([])
  const [status, setStatus] = useState<'all' | ExtractTaskStatus>('all')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [counts, setCounts] = useState({ active: 0, success: 0, failed: 0 })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [deletingId, setDeletingId] = useState<number | null>(null)

  const loadTasks = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({ page: String(page), per_page: String(PER_PAGE) })
      if (status !== 'all') params.set('status', status)
      if (keyword.trim()) params.set('q', keyword.trim())
      const data = await adminRequest<Paged<ExtractHistoryItem>>(`/api/admin/video/extract-tasks?${params.toString()}`, { token })
      setTasks(data?.items ?? [])
      setTotal(data?.total ?? 0)
    } catch (err) {
      const message = err instanceof Error ? err.message : '提取历史加载失败'
      setError(message)
      showError(message)
    } finally {
      setLoading(false)
    }
  }, [keyword, status, page, token])

  // Status-breakdown totals for the summary cards, scoped to the current
  // keyword filter but independent of the selected status tab and page.
  const loadCounts = useCallback(async () => {
    const fetchCount = async (statusValue: string) => {
      const params = new URLSearchParams({ page: '1', per_page: '1', status: statusValue })
      if (keyword.trim()) params.set('q', keyword.trim())
      const data = await adminRequest<Paged<ExtractHistoryItem>>(`/api/admin/video/extract-tasks?${params.toString()}`, { token })
      return data?.total ?? 0
    }
    try {
      const [active, success, failed] = await Promise.all([
        fetchCount('active'),
        fetchCount('success'),
        fetchCount('failed'),
      ])
      setCounts({ active, success, failed })
    } catch {
      // summary is best-effort; ignore errors
    }
  }, [keyword, token])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadTasks()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadTasks])

  useEffect(() => {
    void loadCounts()
  }, [loadCounts])

  const { active: activeCount, success: successCount, failed: failedCount } = counts

  async function deleteTask(task: ExtractHistoryItem) {
    if (isActiveStatus(task.status)) return
    const name = task.video_title || `视频 #${task.video_id}`
    const confirmed = await confirmAction({
      title: '删除提取记录',
      message: `确认删除「${name}」的提取记录？`,
      confirmLabel: '删除',
      variant: 'danger',
    })
    if (!confirmed) return
    setDeletingId(task.id)
    setError('')
    try {
      await adminRequest<unknown>(`/api/admin/video/extract-tasks/${task.id}`, {
        method: 'DELETE',
        token,
      })
      await loadTasks()
      void loadCounts()
      showSuccess('提取记录已删除')
    } catch (err) {
      const message = err instanceof Error ? err.message : '删除提取记录失败'
      setError(message)
      showError(message)
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <section className="stack">
      <section className="panel">
        <div className="section-header">
          <PanelTitle title="提取历史" count={total} />
          <button className="ghost-button" disabled={loading} type="button" onClick={loadTasks}>
            <RefreshCw size={15} className={loading ? 'spin' : undefined} />
            刷新
          </button>
        </div>
        <div className="summary-grid transcode-history-summary">
          <SummaryCard icon={Clock3} label="进行中" value={activeCount} />
          <SummaryCard icon={CheckCircle2} label="成功" value={successCount} />
          <SummaryCard icon={XCircle} label="失败" value={failedCount} />
        </div>
        <div className="history-filter-bar">
          <div className="segmented-tabs" role="tablist" aria-label="提取状态">
            {statusOptions.map((option) => (
              <button
                className={status === option.value ? 'active' : ''}
                key={option.value}
                type="button"
                onClick={() => { setStatus(option.value); setPage(1) }}
              >
                {option.label}
              </button>
            ))}
          </div>
          <label className="history-search">
            <Search size={14} />
            <input
              value={keyword}
              onChange={(event) => { setKeyword(event.target.value); setPage(1) }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void loadTasks()
              }}
              placeholder="搜索视频、ID、来源、错误"
            />
          </label>
          <button className="ghost-button" disabled={loading} type="button" onClick={loadTasks}>
            查询
          </button>
        </div>
        {error && <span className="status error">{error}</span>}
      </section>

      <section className="table-panel">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>视频</th>
                <th>轨道</th>
                <th>状态</th>
                <th>时间</th>
                <th>信息</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((task) => {
                const active = isActiveStatus(task.status)
                return (
                  <tr key={task.id}>
                    <td>
                      <strong>{task.video_title || `视频 #${task.video_id}`}</strong>
                      <small>#{task.video_id}</small>
                    </td>
                    <td>
                      <span className="track-counts">
                        <AudioLines size={13} /> {task.audio_count}
                        <Captions size={13} style={{ marginLeft: 8 }} /> {task.subtitle_count}
                      </span>
                      {task.ready_count > 0 && <small>已生成 {task.ready_count}</small>}
                      {task.failed_count > 0 && <small className="danger-text">失败 {task.failed_count}</small>}
                    </td>
                    <td>
                      <span className={`status-badge ${STATUS_CLASS[task.status] ?? ''}`}>
                        {STATUS_LABEL[task.status] ?? task.status}
                      </span>
                      {active && <Loader size={11} className="spin" style={{ marginLeft: 4 }} />}
                    </td>
                    <td>
                      {formatDateTime(task.created_at)}
                      <small>耗时 {formatElapsed(task)}</small>
                    </td>
                    <td className="history-message-cell">
                      {task.error_message || task.status_message || '—'}
                    </td>
                    <td>
                      <div className="row-actions">
                        {can('video:delete') ? (
                          <button
                            className="danger"
                            disabled={deletingId === task.id || active}
                            title={active ? '进行中的任务不能删除' : '删除记录'}
                            type="button"
                            onClick={() => void deleteTask(task)}
                          >
                            <Trash2 size={13} />
                            删除
                          </button>
                        ) : (
                          <span className="muted-action">—</span>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
              {!loading && tasks.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', color: 'var(--text-subtle)' }}>
                    暂无提取记录
                  </td>
                </tr>
              )}
              {loading && tasks.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', color: 'var(--text-subtle)' }}>
                    加载中...
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <Pagination page={page} perPage={PER_PAGE} total={total} onPage={setPage} />
      </section>
    </section>
  )
}

function SummaryCard({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Clock3
  label: string
  value: number
}) {
  return (
    <div className="summary-card">
      <Icon size={16} />
      <small>{label}</small>
      <strong>{value}</strong>
    </div>
  )
}
