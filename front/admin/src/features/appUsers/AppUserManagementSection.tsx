import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Loader, Pencil, ShieldBan, ShieldCheck, Smartphone, Trash2 } from 'lucide-react'
import type { ApiResponse, AppUser, AppUserForm } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'

const emptyForm: AppUserForm = {
  username: '',
  password: '',
  nickname: '',
  email: '',
  status: 'active',
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

  const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }

  async function load() {
    const res = await fetch('/api/admin/app-users', { headers })
    const json: ApiResponse<AppUser[]> = await res.json()
    if (json.code === 0) setUsers(json.data ?? [])
  }

  useEffect(() => { load() }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (!form.id && !form.username.trim()) return
    setSaving(true)
    try {
      const url = form.id ? `/api/admin/app-users/${form.id}` : '/api/admin/app-users'
      const method = form.id ? 'PUT' : 'POST'
      await fetch(url, { method, headers, body: JSON.stringify(form) })
      setForm(emptyForm)
      load()
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!window.confirm('确认删除该用户？')) return
    await fetch(`/api/admin/app-users/${id}`, { method: 'DELETE', headers })
    if (form.id === id) setForm(emptyForm)
    load()
  }

  async function handleToggleStatus(u: AppUser) {
    const newStatus = u.status === 'banned' ? 'active' : 'banned'
    await fetch(`/api/admin/app-users/${u.id}`, {
      method: 'PUT', headers,
      body: JSON.stringify({ status: newStatus }),
    })
    load()
  }

  const canCreate = can('app_user:create')
  const canEdit = can('app_user:edit')
  const canDelete = can('app_user:delete')

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="App 用户列表" count={users.length} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>用户名</th>
                <th>昵称</th>
                <th>邮箱</th>
                <th>状态</th>
                <th>注册时间</th>
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
            </tbody>
          </table>
        </div>
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
            {(canCreate || canEdit) && (
              <button type="submit" disabled={saving}>
                {saving ? <Loader size={14} className="spin" /> : <Smartphone size={14} />}
                {form.id ? '保存' : '创建'}
              </button>
            )}
            <button type="button" className="secondary" onClick={() => setForm(emptyForm)}>重置</button>
          </div>
        </form>
      </div>
    </section>
  )
}
