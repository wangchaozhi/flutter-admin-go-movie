import { useEffect, useState } from 'react'
import { RefreshCw, Star, Trash2 } from 'lucide-react'

import type { AdminComment, ApiResponse, Paged } from '../../adminTypes'
import { PanelTitle, Pagination } from '../../components/shared'

const PER_PAGE = 20

async function request<T>(url: string, token: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${token}`)
  const res = await fetch(url, { ...init, headers })
  const body = (await res.json()) as ApiResponse<T>
  if (!res.ok || body.code !== 0) throw new Error(body.msg || '请求失败')
  return body.data as T
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function CommentModerationSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [comments, setComments] = useState<AdminComment[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [keyword, setKeyword] = useState('')
  const [appliedKeyword, setAppliedKeyword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const canDelete = can('comment:delete')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({ page: String(page), per_page: String(PER_PAGE) })
      if (appliedKeyword.trim()) params.set('q', appliedKeyword.trim())
      const data = await request<Paged<AdminComment>>(`/api/admin/comments?${params.toString()}`, token)
      setComments(data?.items ?? [])
      setTotal(data?.total ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载评论失败')
    } finally {
      setLoading(false)
    }
  }

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [page, appliedKeyword])

  async function handleDelete(comment: AdminComment) {
    if (!canDelete) return
    if (!window.confirm('确认删除该评论？')) return
    setError('')
    try {
      await request<unknown>(`/api/admin/comments/${comment.id}`, token, { method: 'DELETE' })
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败')
    }
  }

  return (
    <div className="stack">
      <section className="table-panel">
        <div className="section-header">
          <PanelTitle title="评论管理" count={total} />
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
            placeholder="搜索评论内容"
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
                <th>视频</th>
                <th>用户</th>
                <th>评分</th>
                <th>内容</th>
                <th>时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {comments.map((comment) => (
                <tr key={comment.id}>
                  <td>{comment.video_title || `#${comment.video_id}`}</td>
                  <td>{comment.nickname?.trim() || comment.username || `#${comment.user_id}`}</td>
                  <td>
                    {comment.rating > 0 ? (
                      <span className="rating-cell">
                        <Star size={13} /> {comment.rating}
                      </span>
                    ) : (
                      <span className="muted-action">-</span>
                    )}
                  </td>
                  <td>{comment.content || <span className="muted-action">（仅评分）</span>}</td>
                  <td>{formatDateTime(comment.created_at)}</td>
                  <td>
                    {canDelete ? (
                      <button className="danger" type="button" onClick={() => void handleDelete(comment)}>
                        <Trash2 size={13} />
                        删除
                      </button>
                    ) : (
                      <span className="muted-action">无权限</span>
                    )}
                  </td>
                </tr>
              ))}
              {comments.length === 0 && (
                <tr>
                  <td colSpan={6} className="empty-cell">暂无评论</td>
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
