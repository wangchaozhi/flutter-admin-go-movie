import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { ArrowDown, ArrowUp, CalendarClock, Crown, Loader, Minus, Pencil, Plus, Search, ShieldBan, ShieldCheck, Smartphone, Trash2 } from 'lucide-react'
import type { AppUser, AppUserForm, Paged } from '../../adminTypes'
import { PanelTitle, Pagination } from '../../components/shared'
import { adminRequest } from '../../core/adminApi'
import { confirmAction, showError, showSuccess } from '../../core/feedback'

const PER_PAGE = 20

const emptyForm: AppUserForm = {
  username: '',
  password: '',
  nickname: '',
  email: '',
  status: 'active',
}

function vipExpiry(u: AppUser): Date | null {
  if (!u.vip_until) return null
  const until = new Date(u.vip_until)
  return Number.isNaN(until.getTime()) ? null : until
}

function isVipActive(u: AppUser): boolean {
  const until = vipExpiry(u)
  return until != null && until.getTime() > Date.now()
}

export function AppUserManagementSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [users, setUsers] = useState<AppUser[]>([])
  const [form, setForm] = useState<AppUserForm>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [vipDays, setVipDays] = useState(30)
  const [vipBusy, setVipBusy] = useState(false)
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [vipFilter, setVipFilter] = useState<'all' | 'vip' | 'none'>('all')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')

  const hasFilter = debouncedQuery.trim() !== '' || vipFilter !== 'all'

  async function load() {
    const params = new URLSearchParams({
      page: String(page),
      per_page: String(PER_PAGE),
      sort: sortDir,
    })
    const keyword = debouncedQuery.trim()
    if (keyword) params.set('q', keyword)
    if (vipFilter !== 'all') params.set('vip', vipFilter)
    try {
      const data = await adminRequest<Paged<AppUser>>(`/api/admin/app-users?${params.toString()}`, { token })
      setUsers(data?.items ?? [])
      setTotal(data?.total ?? 0)
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载 App 用户失败'
      setError(message)
      showError(message)
    }
  }

  // Debounce the search box and snap back to the first page on a new keyword.
  useEffect(() => {
    const t = setTimeout(() => {
      setDebouncedQuery(query)
      setPage(1)
    }, 300)
    return () => clearTimeout(t)
  }, [query])

  // Reload whenever the page or the effective query window changes.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [page, debouncedQuery, vipFilter, sortDir])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (!form.id && !form.username.trim()) return
    setSaving(true)
    setError('')
    try {
      const url = form.id ? `/api/admin/app-users/${form.id}` : '/api/admin/app-users'
      const method = form.id ? 'PUT' : 'POST'
      await adminRequest<unknown>(url, { method, token, body: JSON.stringify(form) })
      setForm(emptyForm)
      await load()
      showSuccess(form.id ? '用户已保存' : '用户已创建')
    } catch (err) {
      const message = err instanceof Error ? err.message : '保存用户失败'
      setError(message)
      showError(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
    const confirmed = await confirmAction({
      title: '删除 App 用户',
      message: '确认删除该用户？删除后无法恢复。',
      confirmLabel: '删除',
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await adminRequest<unknown>(`/api/admin/app-users/${id}`, { method: 'DELETE', token })
      showSuccess('用户已删除')
    } catch (err) {
      const message = err instanceof Error ? err.message : '删除用户失败'
      setError(message)
      showError(message)
      return
    }
    if (form.id === id) setForm(emptyForm)
    await load()
  }

  async function handleToggleStatus(u: AppUser) {
    const newStatus = u.status === 'banned' ? 'active' : 'banned'
    try {
      await adminRequest<unknown>(`/api/admin/app-users/${u.id}`, {
        method: 'PUT',
        token,
        body: JSON.stringify({ status: newStatus }),
      })
      await load()
      showSuccess(newStatus === 'active' ? '用户已解封' : '用户已封禁')
    } catch (err) {
      const message = err instanceof Error ? err.message : '更新用户状态失败'
      setError(message)
      showError(message)
    }
  }

  async function adjustVip(id: number, days: number) {
    if (vipBusy || days === 0) return
    setVipBusy(true)
    try {
      await adminRequest<unknown>(`/api/admin/app-users/${id}/vip`, {
        method: 'POST',
        token,
        body: JSON.stringify({ days }),
      })
      await load()
      showSuccess(days > 0 ? '会员时长已增加' : '会员时长已减少')
    } catch (err) {
      const message = err instanceof Error ? err.message : '调整会员时长失败'
      setError(message)
      showError(message)
    } finally {
      setVipBusy(false)
    }
  }

  async function clearVip(id: number) {
    const confirmed = await confirmAction({
      title: '清除会员资格',
      message: '确认清除该用户的会员资格？',
      confirmLabel: '清除',
      variant: 'danger',
    })
    if (!confirmed) return
    await adjustVip(id, -100000)
  }

  const canCreate = can('app_user:create')
  const canEdit = can('app_user:edit')
  const canDelete = can('app_user:delete')
  const canSave = form.id ? canEdit : canCreate

  const editingUser = form.id ? users.find(u => u.id === form.id) : undefined

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="App 用户列表" count={total} />
        {error && <span className="status error">{error}</span>}
        <div className="table-filters">
          <div className="table-search">
            <Search size={14} />
            <input
              type="search"
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="搜索 ID / 用户名 / 昵称 / 邮箱"
            />
          </div>
          <select
            className="vip-filter"
            value={vipFilter}
            onChange={e => { setVipFilter(e.target.value as 'all' | 'vip' | 'none'); setPage(1) }}
            aria-label="会员筛选"
          >
            <option value="all">全部会员状态</option>
            <option value="vip">仅会员</option>
            <option value="none">仅非会员</option>
          </select>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>用户名</th>
                <th>昵称</th>
                <th>邮箱</th>
                <th>状态</th>
                <th>会员</th>
                <th
                  className="sortable-th"
                  onClick={() => { setSortDir(d => (d === 'asc' ? 'desc' : 'asc')); setPage(1) }}
                  title="点击切换注册时间排序"
                >
                  注册时间 {sortDir === 'asc' ? <ArrowUp size={12} /> : <ArrowDown size={12} />}
                </th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {users.map(u => (
                <tr key={u.id} className={form.id === u.id ? 'row-active' : ''}>
                  <td className="text-faint">{u.id}</td>
                  <td>{u.username}</td>
                  <td>{u.nickname || '—'}</td>
                  <td className="text-faint">{u.email || '—'}</td>
                  <td>
                    <span className={`status-badge ${u.status === 'active' ? 'status-ready' : 'status-offline'}`}>
                      {u.status === 'active' ? '正常' : '封禁'}
                    </span>
                  </td>
                  <td>
                    {isVipActive(u) ? (
                      <span
                        className="status-badge status-vip"
                        title={`会员到期：${vipExpiry(u)!.toLocaleString()}`}
                      >
                        <Crown size={12} /> 会员
                      </span>
                    ) : (
                      <span className="text-faint">非会员</span>
                    )}
                  </td>
                  <td className="text-faint">{u.created_at ? new Date(u.created_at).toLocaleDateString() : '—'}</td>
                  <td>
                    <div className="row-actions">
                      {canEdit && (
                        <button type="button" onClick={() =>
                          setForm({ id: u.id, username: u.username, password: '', nickname: u.nickname, email: u.email, status: u.status })
                        }>
                          <Pencil size={13} /> 编辑
                        </button>
                      )}
                      {canEdit && (
                        <button type="button" onClick={() => handleToggleStatus(u)}>
                          {u.status === 'active'
                            ? <><ShieldBan size={13} /> 封禁</>
                            : <><ShieldCheck size={13} /> 解封</>}
                        </button>
                      )}
                      {canDelete && (
                        <button className="danger" type="button" onClick={() => handleDelete(u.id)}>
                          <Trash2 size={13} /> 删除
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
              {users.length === 0 && (
                <tr>
                  <td colSpan={8} className="text-faint" style={{ textAlign: 'center', padding: '18px' }}>
                    {hasFilter ? '没有匹配的用户' : '暂无 App 用户'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <Pagination page={page} perPage={PER_PAGE} total={total} onPage={setPage} />
      </section>

      <div className="editor-panel">
        <form onSubmit={handleSave}>
          <PanelTitle title={form.id ? `编辑用户 #${form.id}` : '新增 App 用户'} />

          {!form.id && (
            <label>
              用户名
              <input
                value={form.username}
                onChange={e => setForm({ ...form, username: e.target.value })}
                placeholder="登录用户名"
                required
              />
            </label>
          )}

          <label>
            昵称
            <input
              value={form.nickname}
              onChange={e => setForm({ ...form, nickname: e.target.value })}
              placeholder="显示昵称"
            />
          </label>

          <label>
            邮箱
            <input
              type="email"
              value={form.email}
              onChange={e => setForm({ ...form, email: e.target.value })}
              placeholder="邮箱（可选）"
            />
          </label>

          <label>
            {form.id ? '新密码（留空不修改）' : '密码'}
            <input
              type="password"
              value={form.password}
              onChange={e => setForm({ ...form, password: e.target.value })}
              placeholder={form.id ? '留空不修改' : '登录密码'}
              required={!form.id}
            />
          </label>

          {form.id && (
            <label>
              状态
              <select value={form.status} onChange={e => setForm({ ...form, status: e.target.value as AppUserForm['status'] })}>
                <option value="active">正常</option>
                <option value="banned">封禁</option>
              </select>
            </label>
          )}

          <div className="form-actions">
            {canSave && (
              <button type="submit" disabled={saving}>
                {saving ? <Loader size={14} className="spin" /> : <Smartphone size={14} />}
                {form.id ? '保存' : '创建'}
              </button>
            )}
            <button type="button" className="secondary" onClick={() => setForm(emptyForm)}>
              {form.id ? '取消' : '重置'}
            </button>
          </div>
        </form>

        {canEdit && editingUser && (
          <div className="vip-panel">
            <div className="vip-panel-title"><Crown size={14} /> 会员时长</div>
            <div className="vip-status">
              {isVipActive(editingUser) ? (
                <>
                  当前：
                  <span className="status-badge status-vip"><Crown size={12} /> 会员</span>
                  {' '}到期 {vipExpiry(editingUser)!.toLocaleString()}
                </>
              ) : (
                <span className="text-faint">当前：非会员</span>
              )}
            </div>
            <div className="vip-presets">
              <button type="button" disabled={vipBusy} onClick={() => adjustVip(editingUser.id, 30)}><Plus size={12} /> 1 个月</button>
              <button type="button" disabled={vipBusy} onClick={() => adjustVip(editingUser.id, 90)}><Plus size={12} /> 3 个月</button>
              <button type="button" disabled={vipBusy} onClick={() => adjustVip(editingUser.id, 365)}><Plus size={12} /> 1 年</button>
              <button type="button" disabled={vipBusy} onClick={() => adjustVip(editingUser.id, -30)}><Minus size={12} /> 1 个月</button>
            </div>
            <div className="vip-custom">
              <CalendarClock size={14} />
              <input
                type="number"
                value={vipDays}
                onChange={e => setVipDays(parseInt(e.target.value, 10) || 0)}
              />
              <span className="text-faint">天</span>
              <button
                type="button"
                disabled={vipBusy || vipDays === 0}
                onClick={() => adjustVip(editingUser.id, Math.abs(vipDays))}
              >
                {vipBusy ? <Loader size={12} className="spin" /> : <Plus size={12} />} 增加
              </button>
              <button
                type="button"
                disabled={vipBusy || vipDays === 0}
                onClick={() => adjustVip(editingUser.id, -Math.abs(vipDays))}
              >
                <Minus size={12} /> 减少
              </button>
            </div>
            {isVipActive(editingUser) && (
              <div className="vip-presets" style={{ marginTop: 6 }}>
                <button
                  type="button"
                  className="danger"
                  disabled={vipBusy}
                  onClick={() => void clearVip(editingUser.id)}
                >
                  <Trash2 size={12} /> 清除会员
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  )
}
