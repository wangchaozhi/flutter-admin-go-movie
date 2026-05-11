import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { FolderOpen, Loader, Pencil, Trash2 } from 'lucide-react'
import type { ApiResponse, Category, CategoryForm } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'

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

  const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }

  async function load() {
    const res = await fetch('/api/admin/categories', { headers })
    const json: ApiResponse<Category[]> = await res.json()
    if (json.code === 0) setCategories(json.data ?? [])
  }

  useEffect(() => { load() }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    setSaving(true)
    try {
      const url = form.id ? `/api/admin/categories/${form.id}` : '/api/admin/categories'
      const method = form.id ? 'PUT' : 'POST'
      await fetch(url, { method, headers, body: JSON.stringify(form) })
      setForm(emptyForm)
      load()
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!window.confirm('确认删除该类别？')) return
    await fetch(`/api/admin/categories/${id}`, { method: 'DELETE', headers })
    if (form.id === id) setForm(emptyForm)
    load()
  }

  const canCreate = can('category:create')
  const canEdit = can('category:edit')
  const canDelete = can('category:delete')

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="类别列表" count={categories.length} />
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
            {(canCreate || canEdit) && (
              <button type="submit" disabled={saving}>
                {saving ? <Loader size={14} className="spin" /> : <FolderOpen size={14} />}
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
