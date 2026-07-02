import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'

import type { AuditLog, Paged } from '../../adminTypes'
import { PanelTitle, Pagination } from '../../components/shared'
import { adminRequest } from '../../core/adminApi'
import { showError } from '../../core/feedback'

const PER_PAGE = 20

async function request<T>(url: string, token: string, init: RequestInit = {}): Promise<T> {
  return adminRequest<T>(url, { ...init, token })
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

// Map HTTP status into the shared status-badge palette so successes and
// failures are visually distinct at a glance.
function statusClass(status: number) {
  if (status >= 500) return 'status-failed'
  if (status >= 400) return 'status-offline'
  if (status >= 200 && status < 300) return 'status-ready'
  return 'status-uploaded'
}

export function AuditLogsSection({ token }: { token: string }) {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [keyword, setKeyword] = useState('')
  const [appliedKeyword, setAppliedKeyword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({ page: String(page), per_page: String(PER_PAGE) })
      if (appliedKeyword.trim()) params.set('q', appliedKeyword.trim())
      const data = await request<Paged<AuditLog>>(`/api/admin/audit-logs?${params.toString()}`, token)
      setLogs(data?.items ?? [])
      setTotal(data?.total ?? 0)
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载审计日志失败'
      setError(message)
      showError(message)
    } finally {
      setLoading(false)
    }
  }

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [page, appliedKeyword])

  return (
    <div className="stack">
      <section className="table-panel">
        <div className="section-header">
          <PanelTitle title="审计日志" count={total} />
          <button className="ghost-button" type="button" onClick={load} disabled={loading}>
            <RefreshCw size={15} className={loading ? 'spin' : undefined} />
            刷新
          </button>
        </div>
        {error && <span className="status error">{error}</span>}
        <form
          className="history-filter-bar"
          onSubmit={(e) => {
            e.preventDefault()
            setPage(1)
            setAppliedKeyword(keyword)
          }}
        >
          <input
            className="history-search"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索管理员或路径"
          />
          <button type="submit">搜索</button>
          {appliedKeyword && (
            <button
              type="button"
              className="ghost-button"
              onClick={() => {
                setKeyword('')
                setAppliedKeyword('')
                setPage(1)
              }}
            >
              重置
            </button>
          )}
        </form>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>时间</th>
                <th>管理员</th>
                <th>方法</th>
                <th>路径</th>
                <th>状态</th>
                <th>IP</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <tr key={log.id}>
                  <td>{formatDateTime(log.created_at)}</td>
                  <td>{log.username || <span className="muted-action">未登录</span>}</td>
                  <td>{log.method}</td>
                  <td title={log.path}>{log.path}</td>
                  <td><span className={`status-badge ${statusClass(log.status)}`}>{log.status}</span></td>
                  <td>{log.ip || '-'}</td>
                </tr>
              ))}
              {logs.length === 0 && (
                <tr>
                  <td colSpan={6} className="empty-cell">暂无审计记录</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <Pagination page={page} perPage={PER_PAGE} total={total} onPage={setPage} />
      </section>
    </div>
  )
}
