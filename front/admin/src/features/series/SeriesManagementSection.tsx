import { useEffect, useMemo, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { ImageUp, Layers, Loader, Pencil, Plus, Trash2, X } from 'lucide-react'
import type {
  Category,
  Series,
  SeriesEpisode,
  SeriesForm,
  SeriesStatus,
  Video,
} from '../../adminTypes'
import { PanelTitle } from '../../components/shared'
import { adminRequest } from '../../core/adminApi'
import { confirmAction, showError, showSuccess } from '../../core/feedback'

const emptyForm: SeriesForm = {
  title: '',
  description: '',
  category_id: 0,
  region: '',
  release_year: 0,
  genres: [],
  is_vip: false,
  status: 'ongoing',
}

const statusLabels: Record<SeriesStatus, string> = {
  ongoing: '连载中',
  completed: '已完结',
  offline: '已下架',
}

function joinList(values?: string[]) {
  return (values ?? []).join('、')
}

function parseList(value: string) {
  return value
    .split(/[、,，/]/)
    .map(item => item.trim())
    .filter(Boolean)
}

function seriesToForm(s: Series): SeriesForm {
  return {
    id: s.id,
    title: s.title,
    description: s.description,
    category_id: s.category_id,
    region: s.region,
    release_year: s.release_year,
    genres: s.genres ?? [],
    is_vip: s.is_vip,
    status: s.status,
  }
}

export function SeriesManagementSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [form, setForm] = useState<SeriesForm>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [episodes, setEpisodes] = useState<SeriesEpisode[]>([])
  const [videos, setVideos] = useState<Video[]>([])
  const [pickVideoId, setPickVideoId] = useState(0)
  const [pickEpisodeNumber, setPickEpisodeNumber] = useState(0)
  const [error, setError] = useState('')
  const coverInputRef = useRef<HTMLInputElement | null>(null)

  const canCreate = can('series:create')
  const canEdit = can('series:edit')
  const canDelete = can('series:delete')
  const canSave = form.id ? canEdit : canCreate

  async function loadSeries() {
    try {
      const data = await adminRequest<Series[]>('/api/admin/series', { token })
      setSeriesList(data ?? [])
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载剧集失败'
      setError(message)
      showError(message)
    }
  }

  async function loadCategories() {
    try {
      const data = await adminRequest<Category[]>('/api/admin/categories', { token })
      setCategories(data ?? [])
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载类别失败'
      setError(message)
      showError(message)
    }
  }

  async function loadVideos() {
    try {
      const data = await adminRequest<Video[]>('/api/admin/videos', { token })
      setVideos(data ?? [])
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载视频失败'
      setError(message)
      showError(message)
    }
  }

  async function loadEpisodes(seriesId: number) {
    try {
      const data = await adminRequest<SeriesEpisode[]>(`/api/admin/series/${seriesId}/episodes`, { token })
      setEpisodes(data ?? [])
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载分集失败'
      setError(message)
      showError(message)
    }
  }

  // mount-only load
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { loadSeries(); loadCategories(); loadVideos() }, [])

  useEffect(() => {
    if (form.id) {
      loadEpisodes(form.id)
    } else {
      setEpisodes([])
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form.id])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (!form.title.trim()) return
    setSaving(true)
    setError('')
    try {
      const url = form.id ? `/api/admin/series/${form.id}` : '/api/admin/series'
      const method = form.id ? 'PUT' : 'POST'
      const data = await adminRequest<Series>(url, { method, token, body: JSON.stringify(form) })
      if (!form.id && data) {
        // Stay on the new series so the operator can immediately add episodes.
        setForm(seriesToForm(data))
      }
      await loadSeries()
      showSuccess(form.id ? '剧集已保存' : '剧集已创建')
    } catch (err) {
      const message = err instanceof Error ? err.message : '保存剧集失败'
      setError(message)
      showError(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
    const confirmed = await confirmAction({
      title: '删除剧集',
      message: '确认删除该剧集？其下分集会被解除关联（视频本身保留）。',
      confirmLabel: '删除',
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await adminRequest<unknown>(`/api/admin/series/${id}`, { method: 'DELETE', token })
      showSuccess('剧集已删除')
    } catch (err) {
      const message = err instanceof Error ? err.message : '删除剧集失败'
      setError(message)
      showError(message)
      return
    }
    if (form.id === id) setForm(emptyForm)
    await loadSeries()
  }

  async function uploadCover(file: File | undefined) {
    if (!file || !form.id) return
    setSaving(true)
    setError('')
    try {
      const body = new FormData()
      body.append('file', file)
      await adminRequest<unknown>(`/api/admin/series/${form.id}/cover`, {
        method: 'POST',
        token,
        body,
      })
      await loadSeries()
      showSuccess('封面已上传')
    } catch (err) {
      const message = err instanceof Error ? err.message : '封面上传失败'
      setError(message)
      showError(message)
    } finally {
      setSaving(false)
    }
  }

  async function addEpisode() {
    if (!form.id || !pickVideoId) return
    setError('')
    try {
      await adminRequest<unknown>(`/api/admin/series/${form.id}/episodes`, {
        method: 'POST',
        token,
        body: JSON.stringify({ video_id: pickVideoId, episode_number: pickEpisodeNumber }),
      })
      setPickVideoId(0)
      setPickEpisodeNumber(0)
      await loadEpisodes(form.id)
      await loadVideos()
      showSuccess('分集已添加')
    } catch (err) {
      const message = err instanceof Error ? err.message : '添加分集失败'
      setError(message)
      showError(message)
      return
    }
  }

  async function removeEpisode(videoId: number) {
    if (!form.id) return
    const confirmed = await confirmAction({
      title: '移除分集',
      message: '确认从当前剧集中移除该分集？视频本身会保留。',
      confirmLabel: '移除',
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await adminRequest<unknown>(`/api/admin/series/${form.id}/episodes/${videoId}`, {
        method: 'DELETE',
        token,
      })
      await loadEpisodes(form.id)
      await loadVideos()
      showSuccess('分集已移除')
    } catch (err) {
      const message = err instanceof Error ? err.message : '移除分集失败'
      setError(message)
      showError(message)
    }
  }

  const editingSeries = form.id ? seriesList.find(s => s.id === form.id) : undefined
  // Candidate videos to attach: standalone (unassigned) ready/uploaded videos.
  const assignedIds = useMemo(() => new Set(episodes.map(e => e.id)), [episodes])
  const candidateVideos = useMemo(
    () => videos.filter(v => v.series_id === 0 && !assignedIds.has(v.id)),
    [videos, assignedIds],
  )

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="剧集列表" count={seriesList.length} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>名称</th>
                <th>分集</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {seriesList.map(s => (
                <tr key={s.id} className={form.id === s.id ? 'row-active' : ''}>
                  <td className="text-faint">{s.id}</td>
                  <td>
                    {s.title}
                    {s.is_vip && <span className="pill" style={{ marginLeft: 6 }}>VIP</span>}
                  </td>
                  <td className="text-faint">{s.episode_count ?? 0}</td>
                  <td className="text-faint">{statusLabels[s.status]}</td>
                  <td>
                    <div className="row-actions">
                      <button type="button" onClick={() => setForm(seriesToForm(s))}>
                        <Pencil size={13} /> {canEdit ? '编辑' : '查看'}
                      </button>
                      {canDelete && (
                        <button className="danger" type="button" onClick={() => handleDelete(s.id)}>
                          <Trash2 size={13} /> 删除
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
              {seriesList.length === 0 && (
                <tr><td colSpan={5} className="text-faint">还没有剧集，先在右侧创建一个</td></tr>
              )}
            </tbody>
          </table>
        </div>

        {form.id && (
          <div className="episode-manager" style={{ marginTop: 16 }}>
            <PanelTitle title={`分集管理 · ${editingSeries?.title ?? ''}`} count={episodes.length} />
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>集数</th>
                    <th>标题</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {episodes.map(ep => (
                    <tr key={ep.id}>
                      <td className="text-faint">第 {ep.episode_number} 集</td>
                      <td>{ep.title}</td>
                      <td className="text-faint">{ep.status}{ep.is_vip ? ' · VIP' : ''}</td>
                      <td>
                        {canEdit && (
                          <button className="danger" type="button" onClick={() => removeEpisode(ep.id)}>
                            <X size={13} /> 移除
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                  {episodes.length === 0 && (
                    <tr><td colSpan={4} className="text-faint">该剧集还没有分集</td></tr>
                  )}
                </tbody>
              </table>
            </div>
            {canEdit && (
              <div className="form-split" style={{ marginTop: 10 }}>
                <label>
                  选择视频
                  <select value={pickVideoId} onChange={e => setPickVideoId(Number(e.target.value))}>
                    <option value={0}>从未分配的视频中选择…</option>
                    {candidateVideos.map(v => (
                      <option key={v.id} value={v.id}>#{v.id} {v.title}</option>
                    ))}
                  </select>
                </label>
                <label>
                  集数（留空自动顺延）
                  <input
                    type="number"
                    min={0}
                    value={pickEpisodeNumber || ''}
                    onChange={e => setPickEpisodeNumber(Number(e.target.value))}
                    placeholder="自动"
                  />
                </label>
                <div className="form-actions" style={{ alignItems: 'flex-end' }}>
                  <button type="button" disabled={!pickVideoId} onClick={addEpisode}>
                    <Plus size={14} /> 添加分集
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </section>

      <div className="editor-panel">
        <form onSubmit={handleSave}>
          <PanelTitle title={form.id ? `编辑剧集 #${form.id}` : '新增剧集'} />
          {error && <span className="status error">{error}</span>}

          {form.id && (
            <div className="series-cover-row" style={{ display: 'flex', gap: 12, alignItems: 'center', marginBottom: 8 }}>
              {editingSeries?.cover_url ? (
                <img
                  src={editingSeries.cover_url}
                  alt="cover"
                  style={{ width: 64, height: 90, objectFit: 'cover', borderRadius: 6 }}
                />
              ) : (
                <div style={{ width: 64, height: 90, borderRadius: 6, background: '#1f2937' }} />
              )}
              {canEdit && (
                <>
                  <button type="button" className="secondary" onClick={() => coverInputRef.current?.click()}>
                    <ImageUp size={14} /> 上传封面
                  </button>
                  <input
                    ref={coverInputRef}
                    type="file"
                    accept="image/png,image/jpeg,image/webp"
                    style={{ display: 'none' }}
                    onChange={e => { uploadCover(e.target.files?.[0]); e.target.value = '' }}
                  />
                </>
              )}
            </div>
          )}

          <label>
            名称
            <input
              value={form.title}
              onChange={e => setForm({ ...form, title: e.target.value })}
              placeholder="剧集名称"
              required
            />
          </label>
          <label>
            简介
            <textarea
              value={form.description}
              onChange={e => setForm({ ...form, description: e.target.value })}
              placeholder="剧集简介"
              rows={3}
            />
          </label>
          <label>
            类别
            <select value={form.category_id} onChange={e => setForm({ ...form, category_id: Number(e.target.value) })}>
              <option value={0}>未分类</option>
              {categories.map(c => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </label>
          <div className="form-split">
            <label>
              地区
              <input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })} placeholder="中国大陆" />
            </label>
            <label>
              年份
              <input
                type="number"
                value={form.release_year || ''}
                onChange={e => setForm({ ...form, release_year: Number(e.target.value) })}
                placeholder="2024"
              />
            </label>
          </div>
          <label>
            类型（顿号或逗号分隔）
            <input
              value={joinList(form.genres)}
              onChange={e => setForm({ ...form, genres: parseList(e.target.value) })}
              placeholder="剧情、悬疑、爱情"
            />
          </label>
          <div className="form-split">
            <label>
              状态
              <select value={form.status} onChange={e => setForm({ ...form, status: e.target.value as SeriesStatus })}>
                <option value="ongoing">连载中</option>
                <option value="completed">已完结</option>
                <option value="offline">已下架</option>
              </select>
            </label>
            <label className="checkbox-label" style={{ alignSelf: 'flex-end' }}>
              <input
                type="checkbox"
                checked={form.is_vip}
                onChange={e => setForm({ ...form, is_vip: e.target.checked })}
              />
              VIP 专属
            </label>
          </div>

          <div className="form-actions">
            {canSave && (
              <button type="submit" disabled={saving}>
                {saving ? <Loader size={14} className="spin" /> : <Layers size={14} />}
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
