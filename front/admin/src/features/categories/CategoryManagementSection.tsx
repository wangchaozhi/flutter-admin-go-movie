import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { FolderOpen, Loader, Pencil, Trash2 } from 'lucide-react'
import type { Category, CategoryForm } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'
import { adminRequest } from '../../core/adminApi'
import { confirmAction, showError, showSuccess } from '../../core/feedback'

const emptyForm: CategoryForm = { name: '', sort_order: 0 }

export function CategoryManagementSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [categories, setCategories] = useState<Category[]>([])
  const [form, setForm] = useState<CategoryForm>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function load() {
    try {
      const data = await adminRequest<Category[]>('/api/admin/categories', { token })
      setCategories(data ?? [])
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载类别失败'
      setError(message)
      showError(message)
    }
  }

  // mount-only load; `load` is recreated each render so it stays out of deps
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    setSaving(true)
    setError('')
    try {
      const url = form.id ? `/api/admin/categories/${form.id}` : '/api/admin/categories'
      const method = form.id ? 'PUT' : 'POST'
      await adminRequest<unknown>(url, { method, token, body: JSON.stringify(form) })
      setForm(emptyForm)
      await load()
      showSuccess(form.id ? '类别已保存' : '类别已创建')
    } catch (err) {
      const message = err instanceof Error ? err.message : '保存类别失败'
      setError(message)
      showError(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
    const confirmed = await confirmAction({
      title: '删除类别',
      message: '确认删除该类别？删除后无法恢复。',
      confirmLabel: '删除',
      variant: 'danger',
    })
    if (!confirmed) return
    setError('')
    try {
      await adminRequest<unknown>(`/api/admin/categories/${id}`, { method: 'DELETE', token })
      showSuccess('类别已删除')
    } catch (err) {
      const message = err instanceof Error ? err.message : '删除类别失败'
      setError(message)
      showError(message)
      return
    }
    if (form.id === id) setForm(emptyForm)
    await load()
  }

  const canCreate = can('category:create')
  const canEdit = can('category:edit')
  const canDelete = can('category:delete')
  const canSave = form.id ? canEdit : canCreate

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="类别列表" count={categories.length} />
        {error && <span className="status error">{error}</span>}
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>名称</th>
                <th>排序</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {categories.map(c => (
                <tr key={c.id} className={form.id === c.id ? 'row-active' : ''}>
                  <td className="text-faint">{c.id}</td>
                  <td>{c.name}</td>
                  <td className="text-faint">{c.sort_order}</td>
                  <td>
                    <div className="row-actions">
                      {canEdit && (
                        <button type="button" onClick={() =>
                          setForm({ id: c.id, name: c.name, sort_order: c.sort_order })
                        }>
                          <Pencil size={13} /> 编辑
                        </button>
                      )}
                      {canDelete && (
                        <button className="danger" type="button" onClick={() => handleDelete(c.id)}>
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
          <PanelTitle title={form.id ? `编辑类别 #${form.id}` : '新增类别'} />
          <label>
            名称
            <input
              value={form.name}
              onChange={e => setForm({ ...form, name: e.target.value })}
              placeholder="类别名称"
              required
            />
          </label>
          <label>
            排序值
            <input
              type="number"
              value={form.sort_order}
              onChange={e => setForm({ ...form, sort_order: Number(e.target.value) })}
              placeholder="0"
            />
          </label>
          <div className="form-actions">
            {canSave && (
              <button type="submit" disabled={saving}>
                {saving ? <Loader size={14} className="spin" /> : <FolderOpen size={14} />}
                {form.id ? '保存' : '创建'}
              </button>
            )}
            <button type="button" className="secondary" onClick={() => setForm(emptyForm)}>
              {form.id ? '取消' : '重置'}
            </button>
          </div>
        </form>
      </div>
    </section>
  )
}
