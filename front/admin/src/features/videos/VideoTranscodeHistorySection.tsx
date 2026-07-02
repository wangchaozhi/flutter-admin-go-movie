import { useCallback, useEffect, useState } from 'react'
import {
  CheckCircle2,
  Clock3,
  Loader,
  RefreshCw,
  RotateCcw,
  Search,
  Trash2,
  XCircle,
} from 'lucide-react'

import type { Paged, TranscodeHistoryItem, TranscodeTaskStatus } from '../../adminTypes'
import { PanelTitle, Pagination } from '../../components/shared'
import { adminRequest } from '../../core/adminApi'
import { confirmAction, showError, showSuccess } from '../../core/feedback'

const PER_PAGE = 20

const STATUS_LABEL: Record<TranscodeTaskStatus, string> = {
  queued: '排队中',
  pending: '等待转码',
  processing: '转码中',
  success: '成功',
  failed: '失败',
  canceled: '已取消',
}

const STATUS_CLASS: Record<string, string> = {
  queued: 'status-uploaded',
  pending: 'status-uploaded',
  processing: 'status-transcoding',
  success: 'status-ready',
  failed: 'status-failed',
  canceled: 'status-offline',
}

const statusOptions: Array<{ value: 'all' | TranscodeTaskStatus; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'queued', label: '排队中' },
  { value: 'pending', label: '等待转码' },
  { value: 'processing', label: '转码中' },
  { value: 'success', label: '成功' },
  { value: 'failed', label: '失败' },
  { value: 'canceled', label: '已取消' },
]

const qualityOptions = ['all', '360p', '480p', '720p', '1080p'] as const

function isActiveStatus(status: string) {
  return status === 'queued' || status === 'pending' || status === 'processing'
}

function formatDateTime(value?: string | null) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('zh-CN', { hour12: false })
}

function formatElapsed(task: Pick<TranscodeHistoryItem, 'started_at' | 'finished_at' | 'status'>) {
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

export function VideoTranscodeHistorySection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [tasks, setTasks] = useState<TranscodeHistoryItem[]>([])
  const [status, setStatus] = useState<'all' | TranscodeTaskStatus>('all')
  const [quality, setQuality] = useState<(typeof qualityOptions)[number]>('all')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [counts, setCounts] = useState({ active: 0, success: 0, failed: 0 })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [retryingId, setRetryingId] = useState<number | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)

  const loadTasks = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({ page: String(page), per_page: String(PER_PAGE) })
      if (status !== 'all') params.set('status', status)
      if (quality !== 'all') params.set('quality', quality)
      if (keyword.trim()) params.set('q', keyword.trim())
      const data = await adminRequest<Paged<TranscodeHistoryItem>>(`/api/admin/video/transcode-tasks?${params.toString()}`, { token })
      setTasks(data?.items ?? [])
      setTotal(data?.total ?? 0)
    } catch (err) {
      const message = err instanceof Error ? err.message : '转码历史加载失败'
      setError(message)
      showError(message)
    } finally {
      setLoading(false)
    }
  }, [keyword, quality, status, page, token])

  // Status-breakdown totals for the summary cards, scoped to the current
  // quality/keyword filter but independent of the selected status tab and page.
  const loadCounts = useCallback(async () => {
    const fetchCount = async (statusValue: string) => {
      const params = new URLSearchParams({ page: '1', per_page: '1', status: statusValue })
      if (quality !== 'all') params.set('quality', quality)
      if (keyword.trim()) params.set('q', keyword.trim())
      const data = await adminRequest<Paged<TranscodeHistoryItem>>(`/api/admin/video/transcode-tasks?${params.toString()}`, { token })
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
  }, [keyword, quality, token])

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

  async function retryTask(task: TranscodeHistoryItem) {
    if (!task.quality) return
    setRetryingId(task.id)
    setError('')
    try {
      await adminRequest<unknown>(`/api/admin/videos/${task.video_id}/transcode`, {
        method: 'POST',
        token,
        body: JSON.stringify({ qualities: [task.quality] }),
      })
      await loadTasks()
      void loadCounts()
      showSuccess('重试任务已提交')
    } catch (err) {
      const message = err instanceof Error ? err.message : '重试提交失败'
      setError(message)
      showError(message)
    } finally {
      setRetryingId(null)
    }
  }

  async function deleteTask(task: TranscodeHistoryItem) {
    if (isActiveStatus(task.status)) return
    const name = task.video_title || `视频 #${task.video_id}`
    const confirmed = await confirmAction({
      title: '删除转码记录',
      message: `确认删除「${name}」的 ${task.quality || '未知清晰度'} 转码记录？`,
      confirmLabel: '删除',
      variant: 'danger',
    })
    if (!confirmed) return
    setDeletingId(task.id)
    setError('')
    try {
      await adminRequest<unknown>(`/api/admin/video/transcode-tasks/${task.id}`, {
        method: 'DELETE',
        token,
      })
      await loadTasks()
      void loadCounts()
      showSuccess('转码记录已删除')
    } catch (err) {
      const message = err instanceof Error ? err.message : '删除转码记录失败'
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
          <PanelTitle title="转码历史" count={total} />
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
          <div className="segmented-tabs" role="tablist" aria-label="转码状态">
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
          <select value={quality} onChange={(event) => { setQuality(event.target.value as typeof quality); setPage(1) }}>
            {qualityOptions.map((item) => (
              <option key={item} value={item}>
                {item === 'all' ? '全部清晰度' : item}
              </option>
            ))}
          </select>
          <label className="history-search">
            <Search size={14} />
            <input
              value={keyword}
              onChange={(event) => { setKeyword(event.target.value); setPage(1) }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void loadTasks()
              }}
              placeholder="搜索视频、ID、错误"
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
                <th>清晰度</th>
                <th>状态</th>
                <th>进度</th>
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
                      <small>#{task.video_id} · 批次 {task.batch_id || '—'}</small>
                    </td>
                    <td>
                      <span className="quality-name">{task.quality || '—'}</span>
                      {task.previous_status && <small>原状态 {task.previous_status}</small>}
                    </td>
                    <td>
                      <span className={`status-badge ${STATUS_CLASS[task.status] ?? ''}`}>
                        {STATUS_LABEL[task.status] ?? task.status}
                      </span>
                      {active && <Loader size={11} className="spin" style={{ marginLeft: 4 }} />}
                    </td>
                    <td>
                      <div className="history-progress">
                        <span style={{ width: `${task.progress}%` }} />
                      </div>
                      <small>{task.progress}%</small>
                    </td>
                    <td>
                      {formatDateTime(task.created_at)}
                      <small>耗时 {formatElapsed(task)}</small>
                    </td>
                    <td className="history-message-cell">
                      {task.error_message || task.status_message || '—'}
                      <small>尝试 {task.attempt}</small>
                    </td>
                    <td>
                      <div className="row-actions">
                        {can('video:edit') && task.status === 'failed' && task.quality && (
                          <button
                            disabled={retryingId === task.id}
                            type="button"
                            onClick={() => void retryTask(task)}
                          >
                            <RotateCcw size={13} />
                            重试
                          </button>
                        )}
                        {can('video:delete') && (
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
                        )}
                        {!(can('video:edit') && task.status === 'failed' && task.quality) && !can('video:delete') && (
                          <span className="muted-action">—</span>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
              {!loading && tasks.length === 0 && (
                <tr>
                  <td colSpan={7} style={{ textAlign: 'center', color: 'var(--text-subtle)' }}>
                    暂无转码记录
                  </td>
                </tr>
              )}
              {loading && tasks.length === 0 && (
                <tr>
                  <td colSpan={7} style={{ textAlign: 'center', color: 'var(--text-subtle)' }}>
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
