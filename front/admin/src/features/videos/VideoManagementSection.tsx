import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import {
  Clapperboard,
  CloudUpload,
  Film,
  ImageUp,
  Loader,
  Play,
  RefreshCw,
  Trash2,
} from 'lucide-react'

import type { ApiResponse, Category, TranscodeTask, Video, VideoForm } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'

const emptyForm: VideoForm = {
  title: '',
  description: '',
  category_id: 0,
  is_vip: false,
  is_free: true,
}

const STATUS_LABEL: Record<string, string> = {
  uploading: '上传中',
  uploaded: '待转码',
  transcoding: '转码中',
  ready: '可播放',
  failed: '转码失败',
  offline: '已下架',
}

const STATUS_CLASS: Record<string, string> = {
  uploading: 'status-uploading',
  uploaded: 'status-uploaded',
  transcoding: 'status-transcoding',
  ready: 'status-ready',
  failed: '  status-failed',
  offline: 'status-offline',
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '-'
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return gb.toFixed(2) + ' GB'
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return mb.toFixed(1) + ' MB'
  return (bytes / 1024).toFixed(0) + ' KB'
}

export function VideoManagementSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [videos, setVideos] = useState<Video[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [form, setForm] = useState<VideoForm>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<number | null>(null)
  const [transcoding, setTranscoding] = useState(false)
  const [taskStatus, setTaskStatus] = useState<Record<number, TranscodeTask>>({})
  const [playUrl, setPlayUrl] = useState<string | null>(null)
  const [activeVideoId, setActiveVideoId] = useState<number | null>(null)
  const [uploadError, setUploadError] = useState('')
  const [mp4FileName, setMp4FileName] = useState('')
  const mp4Ref = useRef<HTMLInputElement>(null)
  const coverRef = useRef<HTMLInputElement>(null)

  const authHeader = { Authorization: `Bearer ${token}` }
  const jsonHeaders = { ...authHeader, 'Content-Type': 'application/json' }

  async function loadVideos() {
    const res = await fetch('/api/admin/videos', { headers: jsonHeaders })
    const json: ApiResponse<Video[]> = await res.json()
    if (json.code === 0) setVideos(json.data ?? [])
  }

  async function loadCategories() {
    const res = await fetch('/api/admin/categories', { headers: jsonHeaders })
    const json: ApiResponse<Category[]> = await res.json()
    if (json.code === 0) setCategories(json.data ?? [])
  }

  useEffect(() => { loadVideos(); loadCategories() }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (!form.title.trim()) return
    // capture file before state change unmounts the create-mode input
    const createFile = !form.id ? (mp4Ref.current?.files?.[0] ?? null) : null
    setSaving(true)
    try {
      const url = form.id ? `/api/admin/videos/${form.id}` : '/api/admin/videos'
      const method = form.id ? 'PUT' : 'POST'
      const res = await fetch(url, { method, headers: jsonHeaders, body: JSON.stringify(form) })
      const json: ApiResponse<Video> = await res.json()
      if (method === 'POST' && json.code === 0 && json.data) {
        const v = json.data
        setMp4FileName('')
        setForm({ id: v.id, title: v.title, description: v.description, category_id: v.category_id, is_vip: v.is_vip, is_free: v.is_free })
        if (createFile) handleUploadMp4(v.id, createFile)
      } else if (method === 'PUT') {
        setForm(emptyForm)
      }
      loadVideos()
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!window.confirm('确认删除该视频？')) return
    await fetch(`/api/admin/videos/${id}`, { method: 'DELETE', headers: jsonHeaders })
    if (form.id === id) setForm(emptyForm)
    loadVideos()
  }

  async function handleToggleStatus(v: Video) {
    const newStatus = v.status === 'offline' ? 'ready' : 'offline'
    await fetch(`/api/admin/videos/${v.id}`, {
      method: 'PUT',
      headers: jsonHeaders,
      body: JSON.stringify({ status: newStatus }),
    })
    loadVideos()
  }

  function handleUploadMp4(videoId: number, fileOverride?: File) {
    const file = fileOverride ?? mp4Ref.current?.files?.[0]
    if (!file) { setUploadError('请先选择 MP4 文件'); return }
    setUploadError('')
    setUploadProgress(0)

    const fd = new FormData()
    fd.append('file', file)

    const xhr = new XMLHttpRequest()
    xhr.open('POST', `/api/admin/videos/${videoId}/upload`)
    xhr.setRequestHeader('Authorization', `Bearer ${token}`)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const pct = Math.round((e.loaded / e.total) * 100)
        // cap at 99 so 100% is only shown after server responds
        setUploadProgress(Math.min(pct, 99))
      }
    }

    xhr.onload = () => {
      setUploadProgress(null)
      try {
        const json = JSON.parse(xhr.responseText)
        if (json.code !== 0) {
          setUploadError('上传失败：' + json.msg)
        } else {
          if (mp4Ref.current) mp4Ref.current.value = ''
          loadVideos()
        }
      } catch {
        setUploadError('响应解析失败，请刷新后重试')
      }
    }

    xhr.onerror = () => {
      setUploadProgress(null)
      setUploadError('上传出错，请重试')
    }

    xhr.ontimeout = () => {
      setUploadProgress(null)
      setUploadError('上传超时，请重试')
    }

    xhr.timeout = 10 * 60 * 1000 // 10 分钟

    xhr.send(fd)
  }

  async function handleUploadCover(videoId: number) {
    const file = coverRef.current?.files?.[0]
    if (!file) { setUploadError('请先选择封面图片'); return }
    setUploadError('')
    const fd = new FormData()
    fd.append('file', file)
    const res = await fetch(`/api/admin/videos/${videoId}/cover`, {
      method: 'POST',
      headers: authHeader,
      body: fd,
    })
    const json = await res.json()
    if (json.code !== 0) {
      setUploadError('封面上传失败：' + json.msg)
    } else {
      if (coverRef.current) coverRef.current.value = ''
      loadVideos()
    }
  }

  async function handleTranscode(videoId: number) {
    setTranscoding(true)
    try {
      const res = await fetch(`/api/admin/videos/${videoId}/transcode`, {
        method: 'POST', headers: jsonHeaders,
      })
      const json = await res.json()
      if (json.code !== 0) { alert('转码提交失败：' + json.msg); return }
      loadVideos()
      pollTaskStatus(videoId)
    } finally {
      setTranscoding(false)
    }
  }

  async function pollTaskStatus(videoId: number) {
    const res = await fetch(`/api/admin/videos/${videoId}/transcode`, { headers: jsonHeaders })
    const json: ApiResponse<TranscodeTask> = await res.json()
    if (json.code === 0 && json.data) {
      setTaskStatus(prev => ({ ...prev, [videoId]: json.data! }))
      if (json.data.status === 'processing' || json.data.status === 'pending') {
        setTimeout(() => pollTaskStatus(videoId), 3000)
      } else {
        loadVideos()
      }
    }
  }

  async function handlePlay(videoId: number) {
    setActiveVideoId(videoId)
    const res = await fetch(`/api/videos/${videoId}/play`, { headers: jsonHeaders })
    const json: ApiResponse<{ url: string }> = await res.json()
    if (json.code === 0 && json.data) {
      setPlayUrl(json.data.url)
    } else {
      alert('获取播放地址失败：' + json.msg)
      setActiveVideoId(null)
    }
  }

  const canCreate = can('video:create')
  const canEdit = can('video:edit')
  const canDelete = can('video:delete')
  const uploading = uploadProgress !== null

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="视频列表" count={videos.length} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>标题</th>
                <th>类别</th>
                <th>状态</th>
                <th>大小</th>
                <th>VIP</th>
                <th>免费</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {videos.map(v => {
                const task = taskStatus[v.id]
                return (
                  <tr key={v.id} className={form.id === v.id ? 'row-active' : ''}>
                    <td className="text-faint">{v.id}</td>
                    <td>{v.title}</td>
                    <td className="text-faint">{categories.find(c => c.id === v.category_id)?.name ?? '—'}</td>
                    <td>
                      <span className={`status-badge ${STATUS_CLASS[v.status] ?? ''}`}>
                        {STATUS_LABEL[v.status] ?? v.status}
                      </span>
                      {task && (task.status === 'pending' || task.status === 'processing') && (
                        <Loader size={12} className="spin" style={{ marginLeft: 4 }} />
                      )}
                    </td>
                    <td className="text-faint">{formatBytes(v.size)}</td>
                    <td>{v.is_vip ? '✓' : '—'}</td>
                    <td>{v.is_free ? '✓' : '—'}</td>
                    <td>
                      <div className="row-actions">
                        {canEdit && (
                          <button type="button" onClick={() => {
                            setUploadError('')
                            setForm({ id: v.id, title: v.title, description: v.description, category_id: v.category_id, is_vip: v.is_vip, is_free: v.is_free })
                          }}>
                            <Film size={13} /> 编辑
                          </button>
                        )}
                        {canEdit && v.status === 'uploaded' && (
                          <button type="button" onClick={() => handleTranscode(v.id)} disabled={transcoding}>
                            <RefreshCw size={13} /> 转码
                          </button>
                        )}
                        {canEdit && v.status === 'failed' && (
                          <button type="button" onClick={() => handleTranscode(v.id)} disabled={transcoding}>
                            <RefreshCw size={13} /> 重试
                          </button>
                        )}
                        {canEdit && v.status === 'ready' && (
                          <button type="button" onClick={() => handlePlay(v.id)}>
                            <Play size={13} /> 播放地址
                          </button>
                        )}
                        {canEdit && (v.status === 'ready' || v.status === 'offline') && (
                          <button type="button" onClick={() => handleToggleStatus(v)}>
                            {v.status === 'offline' ? '上架' : '下架'}
                          </button>
                        )}
                        {canDelete && (
                          <button className="danger" type="button" onClick={() => handleDelete(v.id)}>
                            <Trash2 size={13} /> 删除
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>

        {playUrl && activeVideoId && (
          <div className="play-url-box">
            <div className="play-url-header">
              <span>视频 #{activeVideoId} 播放地址（6小时有效）</span>
              <button type="button" onClick={() => { setPlayUrl(null); setActiveVideoId(null) }}>关闭</button>
            </div>
            <input readOnly value={playUrl} onClick={e => (e.target as HTMLInputElement).select()} />
          </div>
        )}
      </section>

      <div className="editor-panel">
        <form onSubmit={handleSave}>
          <PanelTitle title={form.id ? `编辑视频 #${form.id}` : '新增视频'} />
          <label>
            标题
            <input value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} placeholder="视频标题" required />
          </label>
          <label>
            简介
            <textarea
              value={form.description}
              onChange={e => setForm({ ...form, description: e.target.value })}
              placeholder="视频简介"
              rows={3}
              style={{ resize: 'vertical' }}
            />
          </label>
          <label>
            类别
            <select
              value={form.category_id}
              onChange={e => setForm({ ...form, category_id: Number(e.target.value) })}
            >
              <option value={0}>— 不分类 —</option>
              {categories.map(c => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </label>
          <div className="checkbox-row">
            <label className="inline-check">
              <input type="checkbox" checked={form.is_vip} onChange={e => setForm({ ...form, is_vip: e.target.checked })} />
              VIP 专属
            </label>
            <label className="inline-check">
              <input type="checkbox" checked={form.is_free} onChange={e => setForm({ ...form, is_free: e.target.checked })} />
              免费观看
            </label>
          </div>
          {!form.id && (
            <label>
              视频文件 <span style={{ color: 'var(--danger-text)', fontWeight: 600 }}>*</span>
              <input
                ref={mp4Ref}
                type="file"
                accept="video/mp4"
                required
                onChange={e => setMp4FileName(e.target.files?.[0]?.name ?? '')}
              />
              {mp4FileName && <span style={{ fontSize: 12, color: 'var(--text-faint)', marginTop: 4, display: 'block' }}>{mp4FileName}</span>}
            </label>
          )}
          <div className="form-actions">
            {(canCreate || canEdit) && (
              <button type="submit" disabled={saving || (!form.id && !mp4FileName)}>
                {saving ? <Loader size={14} className="spin" /> : <Clapperboard size={14} />}
                {form.id ? '保存' : '创建'}
              </button>
            )}
            <button type="button" className="secondary" onClick={() => { setForm(emptyForm); setUploadError(''); setMp4FileName('') }}>重置</button>
          </div>
        </form>

        {form.id && (
          <>
            <hr style={{ margin: '16px 0', borderColor: 'var(--border)' }} />
            <PanelTitle title="上传文件" />

            {uploadError && (
              <p style={{ color: 'var(--danger-text)', fontSize: 13, marginBottom: 8 }}>{uploadError}</p>
            )}

            <div className="upload-row">
              <label className="upload-label">MP4 视频</label>
              <input ref={mp4Ref} type="file" accept="video/mp4" disabled={uploading} />
              <button type="button" disabled={uploading} onClick={() => handleUploadMp4(form.id!)}>
                <Loader size={13} className={uploading ? 'spin' : undefined} style={{ display: uploading ? undefined : 'none' }} />
                {!uploading && <CloudUpload size={13} />}
                {uploading
                  ? (uploadProgress !== null && uploadProgress < 99 ? `上传 ${uploadProgress}%` : '服务器处理中…')
                  : '上传 MP4'}
              </button>
            </div>

            {uploading && (
              <div className="upload-progress-bar">
                <div className="upload-progress-fill" style={{ width: `${uploadProgress ?? 99}%` }} />
              </div>
            )}

            <div className="upload-row">
              <label className="upload-label">封面图片</label>
              <input ref={coverRef} type="file" accept="image/*" />
              <button type="button" onClick={() => handleUploadCover(form.id!)}>
                <ImageUp size={13} /> 上传封面
              </button>
            </div>
          </>
        )}
      </div>
    </section>
  )
}
