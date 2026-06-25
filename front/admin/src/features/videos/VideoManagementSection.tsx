import { Fragment, useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import {
  ChevronDown,
  ChevronRight,
  Clapperboard,
  CloudUpload,
  Film,
  ImageUp,
  Loader,
  Play,
  RefreshCw,
  Sparkles,
  Trash2,
  XCircle,
} from 'lucide-react'

import type {
  ApiResponse,
  Category,
  Paged,
  TranscodeTask,
  Video,
  VideoAIMetadata,
  VideoForm,
  VideoQualityTask,
} from '../../adminTypes'
import { PanelTitle, Pagination } from '../../components/shared'

const PER_PAGE = 20

const emptyForm: VideoForm = {
  title: '',
  description: '',
  category_id: 0,
  actors: [],
  directors: [],
  genres: [],
  region: '',
  release_year: 0,
  language: '',
  is_vip: false,
  is_free: true,
}

const STATUS_LABEL: Record<string, string> = {
  uploading: '上传中',
  extracting: '处理音轨中',
  uploaded: '待转码',
  transcoding: '转码中',
  ready: '可播放',
  failed: '转码失败',
  offline: '已下架',
}

const STATUS_CLASS: Record<string, string> = {
  uploading: 'status-uploading',
  extracting: 'status-transcoding',
  uploaded: 'status-uploaded',
  transcoding: 'status-transcoding',
  ready: 'status-ready',
  failed: '  status-failed',
  offline: 'status-offline',
}

const TRANSCODE_QUALITIES = ['360p', '480p', '720p', '1080p']

const QUALITY_STATUS_LABEL: Record<string, string> = {
  queued: '排队中',
  pending: '等待转码',
  processing: '转码中',
  success: '已完成',
  failed: '失败',
  canceled: '已取消',
}

const QUALITY_STATUS_CLASS: Record<string, string> = {
  queued: 'status-uploaded',
  pending: 'status-uploaded',
  processing: 'status-transcoding',
  success: 'status-ready',
  failed: 'status-failed',
  canceled: 'status-offline',
}

function formatDateTime(value?: string | null) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('zh-CN', { hour12: false })
}

function formatElapsed(task: Pick<VideoQualityTask, 'started_at' | 'finished_at' | 'status'>) {
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

function formatBytes(bytes: number) {
  if (bytes === 0) return '-'
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return gb.toFixed(2) + ' GB'
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return mb.toFixed(1) + ' MB'
  return (bytes / 1024).toFixed(0) + ' KB'
}

// duration is stored in seconds and probed on upload; 0 means not yet probed.
function formatDuration(seconds: number) {
  if (!seconds || seconds <= 0) return '—'
  const total = Math.round(seconds)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const mm = String(m).padStart(2, '0')
  const ss = String(s).padStart(2, '0')
  return h > 0 ? `${h}:${mm}:${ss}` : `${m}:${ss}`
}

function isActiveTranscodeStatus(status?: string) {
  return status === 'queued' || status === 'pending' || status === 'processing'
}

function effectiveQualityStatus(status: string | undefined, transcoded?: boolean) {
  return transcoded && status === 'canceled' ? 'success' : status
}

function joinCatalogList(values?: string[]) {
  return (values ?? []).join('、')
}

function parseCatalogList(value: string) {
  return value
    .split(/[、,，/]/)
    .map(item => item.trim())
    .filter(Boolean)
}

function videoToForm(v: Video): VideoForm {
  return {
    id: v.id,
    title: v.title,
    description: v.description,
    category_id: v.category_id,
    actors: v.actors ?? [],
    directors: v.directors ?? [],
    genres: v.genres ?? [],
    region: v.region ?? '',
    release_year: v.release_year ?? 0,
    language: v.language ?? '',
    is_vip: v.is_vip,
    is_free: v.is_free,
  }
}

function transcodeStateLabel(status: string | undefined, fallback: string, progress?: number) {
  if (status === 'failed') return '失败'
  if (status === 'canceled') return '已取消'
  if (status === 'success') return '已转'
  if (status === 'queued') return '等待入队'
  if (status === 'pending') return '等待转码'
  if (status === 'processing') return `${fallback || '转码中'} ${progress ?? 0}%`
  return fallback
}

export function VideoManagementSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const [videos, setVideos] = useState<Video[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [categories, setCategories] = useState<Category[]>([])
  const [form, setForm] = useState<VideoForm>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<number | null>(null)
  const [transcoding, setTranscoding] = useState(false)
  const [generatingMetadataId, setGeneratingMetadataId] = useState<number | null>(null)
  const [cancelingTranscode, setCancelingTranscode] = useState<Set<string>>(new Set())
  const [transcodeDialog, setTranscodeDialog] = useState<{
    video: Video
    selected: string[]
  } | null>(null)
  const [taskStatus, setTaskStatus] = useState<Record<number, TranscodeTask>>({})
  const [qualityTasks, setQualityTasks] = useState<Record<number, VideoQualityTask[]>>({})
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [qualityLoading, setQualityLoading] = useState<Set<number>>(new Set())
  const [playUrl, setPlayUrl] = useState<string | null>(null)
  const [activeVideoId, setActiveVideoId] = useState<number | null>(null)
  const [uploadError, setUploadError] = useState('')
  const [mp4FileName, setMp4FileName] = useState('')
  const mp4Ref = useRef<HTMLInputElement>(null)
  const coverRef = useRef<HTMLInputElement>(null)
  const uploadXhrRef = useRef<XMLHttpRequest | null>(null)
  const uploadVideoIdRef = useRef<number | null>(null)
  const pollingTranscodesRef = useRef<Set<number>>(new Set())
  // keep the latest expanded set available to the polling closure
  const expandedRef = useRef<Set<number>>(expanded)
  useEffect(() => { expandedRef.current = expanded }, [expanded])

  const authHeader = { Authorization: `Bearer ${token}` }
  const jsonHeaders = { ...authHeader, 'Content-Type': 'application/json' }

  async function loadVideos(): Promise<Video[]> {
    const params = new URLSearchParams({ page: String(page), per_page: String(PER_PAGE) })
    const res = await fetch(`/api/admin/videos?${params.toString()}`, { headers: jsonHeaders })
    const json: ApiResponse<Paged<Video>> = await res.json()
    const list = json.code === 0 && json.data ? (json.data.items ?? []) : []
    if (json.code === 0 && json.data) {
      setVideos(list)
      setTotal(json.data.total ?? 0)
    }
    return list
  }

  // After upload the source's audio/subtitle tracks are extracted in the
  // background (status "extracting"); refresh the list until it settles.
  function pollExtractionStatus(videoId: number) {
    setTimeout(async () => {
      const list = await loadVideos()
      const v = list.find(item => item.id === videoId)
      if (v && v.status === 'extracting') pollExtractionStatus(videoId)
    }, 3000)
  }

  async function loadCategories() {
    const res = await fetch('/api/admin/categories', { headers: jsonHeaders })
    const json: ApiResponse<Category[]> = await res.json()
    if (json.code === 0) setCategories(json.data ?? [])
  }

  // mount-only category load; loaders are recreated each render so they stay out of deps
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { loadCategories() }, [])

  // (Re)load the video list on mount and whenever the page changes.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { loadVideos() }, [page])

  // If the page is refreshed during a merge re-transcode, videos can stay
  // "ready" while the list API reports the active transcode separately.
  useEffect(() => {
    for (const video of videos) {
      const task = taskStatus[video.id]
      if ((video.transcoding || video.status === 'transcoding') && (!task || isActiveTranscodeStatus(task.status))) {
        startTaskPolling(video.id)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [videos, taskStatus])

  function transcodeCancelKey(videoId: number, quality?: string) {
    return quality ? `${videoId}:${quality}` : `${videoId}:all`
  }

  function clearActiveUpload(xhr: XMLHttpRequest) {
    if (uploadXhrRef.current === xhr) {
      uploadXhrRef.current = null
      uploadVideoIdRef.current = null
    }
  }

  function cancelUploadIfActive(videoId?: number) {
    if (!uploadXhrRef.current) return false
    if (videoId !== undefined && uploadVideoIdRef.current !== videoId) return false
    uploadXhrRef.current.abort()
    return true
  }

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
        setForm(videoToForm(v))
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
    cancelUploadIfActive(id)
    const res = await fetch(`/api/admin/videos/${id}`, { method: 'DELETE', headers: jsonHeaders })
    const json: ApiResponse<unknown> = await res.json()
    if (json.code !== 0) { alert('删除失败：' + json.msg); return }
    if (form.id === id) setForm(emptyForm)
    setTaskStatus(prev => {
      const next = { ...prev }
      delete next[id]
      return next
    })
    setQualityTasks(prev => {
      const next = { ...prev }
      delete next[id]
      return next
    })
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
    uploadXhrRef.current = xhr
    uploadVideoIdRef.current = videoId
    xhr.open('POST', `/api/admin/videos/${videoId}/upload`)
    xhr.setRequestHeader('Authorization', `Bearer ${token}`)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const pct = Math.round((e.loaded / e.total) * 100)
        // cap at 99 so 100% is only shown after server responds
        setUploadProgress(Math.min(pct, 99))
      }
    }

    xhr.onload = async () => {
      clearActiveUpload(xhr)
      setUploadProgress(null)
      try {
        const json = JSON.parse(xhr.responseText)
        if (json.code !== 0) {
          setUploadError('上传失败：' + json.msg)
        } else {
          if (mp4Ref.current) mp4Ref.current.value = ''
          const list = await loadVideos()
          const v = list.find(item => item.id === videoId)
          if (v && v.status === 'extracting') pollExtractionStatus(videoId)
        }
      } catch {
        setUploadError('响应解析失败，请刷新后重试')
      }
    }

    xhr.onerror = () => {
      clearActiveUpload(xhr)
      setUploadProgress(null)
      setUploadError('上传出错，请重试')
    }

    xhr.ontimeout = () => {
      clearActiveUpload(xhr)
      setUploadProgress(null)
      setUploadError('上传超时，请重试')
    }

    xhr.onabort = () => {
      clearActiveUpload(xhr)
      setUploadProgress(null)
      setUploadError('上传已取消')
    }

    xhr.timeout = 30 * 60 * 1000 // 30 分钟

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

  function openTranscodeDialog(video: Video) {
    const done = new Set(video.transcoded_qualities ?? [])
    const available = new Set(video.available_transcode_qualities ?? [])
    const canSelectQuality = (quality: string) => available.size === 0 || available.has(quality)
    const latestTask = taskStatus[video.id]
    const failedQualities = TRANSCODE_QUALITIES.filter(quality => latestTask?.quality_statuses?.[quality] === 'failed' && canSelectQuality(quality))
    const selected = failedQualities.length > 0
      ? failedQualities
      : (video.status === 'ready' || video.status === 'offline')
        ? (done.size > 0 ? TRANSCODE_QUALITIES.filter(quality => !done.has(quality) && canSelectQuality(quality)) : [])
        : TRANSCODE_QUALITIES.filter(canSelectQuality)
    setTranscodeDialog({ video, selected })
  }

  function toggleTranscodeQuality(quality: string) {
    setTranscodeDialog(prev => {
      if (!prev) return prev
      const selected = prev.selected.includes(quality)
        ? prev.selected.filter(item => item !== quality)
        : [...prev.selected, quality]
      return { ...prev, selected }
    })
  }

  async function handleTranscode(videoId: number, qualities: string[]) {
    setTranscoding(true)
    try {
      const res = await fetch(`/api/admin/videos/${videoId}/transcode`, {
        method: 'POST',
        headers: jsonHeaders,
        body: JSON.stringify({ qualities }),
      })
      const json = await res.json()
      if (json.code !== 0) { alert('转码提交失败：' + json.msg); return }
      setTranscodeDialog(null)
      loadVideos()
      startTaskPolling(videoId)
    } finally {
      setTranscoding(false)
    }
  }

  async function handleCancelTranscode(videoId: number, quality?: string) {
    const key = transcodeCancelKey(videoId, quality)
    setCancelingTranscode(prev => new Set(prev).add(key))
    try {
      const url = quality
        ? `/api/admin/videos/${videoId}/tasks/${quality}/cancel`
        : `/api/admin/videos/${videoId}/transcode`
      const res = await fetch(url, { method: 'DELETE', headers: jsonHeaders })
      const json: ApiResponse<{ canceled: number }> = await res.json()
      if (json.code !== 0) { alert('取消失败：' + json.msg); return }
      await loadVideos()
      await pollTaskStatus(videoId)
      if (expandedRef.current.has(videoId)) await loadQualityTasks(videoId)
    } finally {
      setCancelingTranscode(prev => {
        const next = new Set(prev)
        next.delete(key)
        return next
      })
    }
  }

  async function pollTaskStatus(videoId: number) {
    try {
      const res = await fetch(`/api/admin/videos/${videoId}/transcode`, { headers: jsonHeaders })
      const json: ApiResponse<TranscodeTask> = await res.json()
      if (json.code !== 0 || !json.data) {
        pollingTranscodesRef.current.delete(videoId)
        await loadVideos()
        return
      }
      setTaskStatus(prev => ({ ...prev, [videoId]: json.data! }))
      if (expandedRef.current.has(videoId)) loadQualityTasks(videoId)
      if (isActiveTranscodeStatus(json.data.status)) {
        setTimeout(() => {
          pollTaskStatus(videoId).catch(() => {
            pollingTranscodesRef.current.delete(videoId)
          })
        }, 3000)
      } else {
        pollingTranscodesRef.current.delete(videoId)
        await loadVideos()
      }
    } catch (error) {
      pollingTranscodesRef.current.delete(videoId)
      throw error
    }
  }

  function startTaskPolling(videoId: number) {
    if (pollingTranscodesRef.current.has(videoId)) return
    pollingTranscodesRef.current.add(videoId)
    pollTaskStatus(videoId).catch(() => {
      pollingTranscodesRef.current.delete(videoId)
    })
  }

  async function loadQualityTasks(videoId: number) {
    setQualityLoading(prev => new Set(prev).add(videoId))
    try {
      const res = await fetch(`/api/admin/videos/${videoId}/tasks`, { headers: jsonHeaders })
      const json: ApiResponse<VideoQualityTask[]> = await res.json()
      if (json.code === 0) setQualityTasks(prev => ({ ...prev, [videoId]: json.data ?? [] }))
    } finally {
      setQualityLoading(prev => {
        const next = new Set(prev)
        next.delete(videoId)
        return next
      })
    }
  }

  function toggleQualityDetail(videoId: number) {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(videoId)) {
        next.delete(videoId)
      } else {
        next.add(videoId)
        loadQualityTasks(videoId)
      }
      return next
    })
  }

  async function handleRetranscodeQuality(videoId: number, quality: string) {
    setTranscoding(true)
    try {
      const res = await fetch(`/api/admin/videos/${videoId}/transcode`, {
        method: 'POST',
        headers: jsonHeaders,
        body: JSON.stringify({ qualities: [quality] }),
      })
      const json = await res.json()
      if (json.code !== 0) { alert('重转提交失败：' + json.msg); return }
      await loadVideos()
      startTaskPolling(videoId)
      loadQualityTasks(videoId)
    } finally {
      setTranscoding(false)
    }
  }

  async function handleDeleteQuality(videoId: number, quality: string) {
    if (!window.confirm(`确认删除 ${quality} 清晰度？该分辨率的播放文件将被移除。`)) return
    const res = await fetch(`/api/admin/videos/${videoId}/tasks/${quality}`, {
      method: 'DELETE',
      headers: jsonHeaders,
    })
    const json = await res.json()
    if (json.code !== 0) { alert('删除失败：' + json.msg); return }
    await loadVideos()
    loadQualityTasks(videoId)
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

  async function handleGenerateMetadata(video: Pick<Video, 'id' | 'description'>) {
    if (!video.id || generatingMetadataId !== null) return
    const overwrite = video.description.trim() !== ''
      ? window.confirm('当前简介已有内容，是否用 AI 生成内容覆盖？')
      : false
    if (video.description.trim() !== '' && !overwrite) return
    setGeneratingMetadataId(video.id)
    try {
      const res = await fetch(`/api/admin/videos/${video.id}/ai-metadata`, {
        method: 'POST',
        headers: jsonHeaders,
        body: JSON.stringify({ overwrite_description: overwrite }),
      })
      const json: ApiResponse<VideoAIMetadata> = await res.json()
      if (json.code !== 0 || !json.data) {
        alert('AI 补全失败：' + json.msg)
        return
      }
      setForm(prev => prev.id === video.id ? { ...prev, description: json.data!.synopsis } : prev)
      await loadVideos()
    } finally {
      setGeneratingMetadataId(null)
    }
  }

  const canCreate = can('video:create')
  const canEdit = can('video:edit')
  const canDelete = can('video:delete')
  const uploading = uploadProgress !== null

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="视频列表" count={total} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>标题</th>
                <th>类别</th>
                <th>状态</th>
                <th>时长</th>
                <th>大小</th>
                <th>VIP</th>
                <th>免费</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {videos.map(v => {
                const task = taskStatus[v.id]
                const isExpanded = expanded.has(v.id)
                const detailTasks = qualityTasks[v.id]
                const detailLoading = qualityLoading.has(v.id)
                const activeTask = task ? isActiveTranscodeStatus(task.status) : false
                const activeTranscode = v.transcoding || v.status === 'transcoding' || activeTask
                const transcodeBadgeLabel = activeTask
                  ? transcodeStateLabel(task.status, task.status_message || '转码中', task.progress)
                  : '转码中'
                const cancelingVideoTranscode = cancelingTranscode.has(transcodeCancelKey(v.id))
                return (
                  <Fragment key={v.id}>
                  <tr className={form.id === v.id ? 'row-active' : ''}>
                    <td className="text-faint">{v.id}</td>
                    <td>{v.title}</td>
                    <td className="text-faint">{categories.find(c => c.id === v.category_id)?.name ?? '—'}</td>
                    <td>
                      <span className={`status-badge ${STATUS_CLASS[v.status] ?? ''}`}>
                        {STATUS_LABEL[v.status] ?? v.status}
                      </span>
                      {activeTranscode && v.status !== 'transcoding' && (
                        <span className="status-badge status-transcoding transcode-status-badge">
                          {transcodeBadgeLabel}
                        </span>
                      )}
                      {activeTranscode && (
                        <Loader size={12} className="spin" style={{ marginLeft: 4 }} />
                      )}
                      {activeTask && (
                        <div className="transcode-inline-progress">
                          {task.status_message || '等待转码'}{task.progress > 0 ? ` ${task.progress}%` : ''}
                        </div>
                      )}
                    </td>
                    <td className="text-faint">{formatDuration(v.duration)}</td>
                    <td className="text-faint">{formatBytes(v.size)}</td>
                    <td>{v.is_vip ? '✓' : '—'}</td>
                    <td>{v.is_free ? '✓' : '—'}</td>
                    <td>
                      <div className="row-actions">
                        <button
                          type="button"
                          className="quality-toggle"
                          aria-expanded={isExpanded}
                          onClick={() => toggleQualityDetail(v.id)}
                        >
                          {isExpanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />} 清晰度
                        </button>
                        {canEdit && (
                          <button type="button" onClick={() => {
                            setUploadError('')
                            setForm(videoToForm(v))
                          }}>
                            <Film size={13} /> 编辑
                          </button>
                        )}
                        {canEdit && (
                          <button type="button" onClick={() => handleGenerateMetadata(v)} disabled={generatingMetadataId !== null}>
                            {generatingMetadataId === v.id ? <Loader size={13} className="spin" /> : <Sparkles size={13} />}
                            AI补全
                          </button>
                        )}
                        {canEdit && v.status === 'uploaded' && (
                          <button type="button" onClick={() => openTranscodeDialog(v)} disabled={transcoding}>
                            <RefreshCw size={13} /> 转码
                          </button>
                        )}
                        {canEdit && v.status === 'failed' && (
                          <button type="button" onClick={() => openTranscodeDialog(v)} disabled={transcoding}>
                            <RefreshCw size={13} /> 重试
                          </button>
                        )}
                        {canEdit && (v.status === 'ready' || v.status === 'offline') && (
                          <button type="button" onClick={() => openTranscodeDialog(v)} disabled={transcoding}>
                            <RefreshCw size={13} /> 继续转码
                          </button>
                        )}
                        {canEdit && activeTranscode && (
                          <button
                            type="button"
                            className="danger"
                            disabled={cancelingVideoTranscode}
                            onClick={() => handleCancelTranscode(v.id)}
                          >
                            {cancelingVideoTranscode ? <Loader size={13} className="spin" /> : <XCircle size={13} />}
                            取消转码
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
                  {isExpanded && (
                    <tr className="quality-detail-row">
                      <td colSpan={9}>
                        {detailLoading && !detailTasks ? (
                          <div className="quality-empty"><Loader size={13} className="spin" /> 加载中…</div>
                        ) : !detailTasks || detailTasks.length === 0 ? (
                          <div className="quality-empty">暂无清晰度转码记录</div>
                        ) : (
                          <table className="quality-table">
                            <thead>
                              <tr>
                                <th>清晰度</th>
                                <th>开始时间</th>
                                <th>结束时间</th>
                                <th>耗时</th>
                                <th>状态</th>
                                <th>操作</th>
                              </tr>
                            </thead>
                            <tbody>
                              {detailTasks.map(qt => {
                                const active = isActiveTranscodeStatus(qt.status)
                                const cancelingQuality = cancelingTranscode.has(transcodeCancelKey(v.id, qt.quality))
                                const displayStatus = effectiveQualityStatus(qt.status, qt.transcoded)
                                const label = QUALITY_STATUS_LABEL[displayStatus ?? ''] ?? displayStatus
                                const statusText = displayStatus === 'processing' && qt.progress > 0
                                  ? `${label} ${qt.progress}%`
                                  : label
                                return (
                                  <tr key={qt.quality}>
                                    <td>
                                      <span className="quality-name">{qt.quality}</span>
                                      {qt.transcoded && <span className="quality-ok-dot" title="已生成可播放文件" />}
                                    </td>
                                    <td className="text-faint">{formatDateTime(qt.started_at)}</td>
                                    <td className="text-faint">{formatDateTime(qt.finished_at)}</td>
                                    <td className="text-faint">{formatElapsed(qt)}</td>
                                    <td>
                                      <span className={`status-badge ${QUALITY_STATUS_CLASS[displayStatus ?? ''] ?? ''}`}>
                                        {statusText}
                                      </span>
                                      {active && <Loader size={11} className="spin" style={{ marginLeft: 4 }} />}
                                      {qt.status === 'failed' && qt.error_message && (
                                        <div className="quality-error" title={qt.error_message}>{qt.error_message}</div>
                                      )}
                                    </td>
                                    <td>
                                      <div className="row-actions">
                                        {canEdit && (
                                          <button
                                            type="button"
                                            disabled={transcoding || active}
                                            onClick={() => handleRetranscodeQuality(v.id, qt.quality)}
                                          >
                                            <RefreshCw size={12} /> 重转
                                          </button>
                                        )}
                                        {canEdit && active && (
                                          <button
                                            type="button"
                                            className="danger"
                                            disabled={cancelingQuality}
                                            onClick={() => handleCancelTranscode(v.id, qt.quality)}
                                          >
                                            {cancelingQuality ? <Loader size={12} className="spin" /> : <XCircle size={12} />}
                                            取消
                                          </button>
                                        )}
                                        {canDelete && (
                                          <button
                                            type="button"
                                            className="danger"
                                            disabled={active}
                                            onClick={() => handleDeleteQuality(v.id, qt.quality)}
                                          >
                                            <Trash2 size={12} /> 删除
                                          </button>
                                        )}
                                      </div>
                                    </td>
                                  </tr>
                                )
                              })}
                            </tbody>
                          </table>
                        )}
                      </td>
                    </tr>
                  )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>
        </div>
        <Pagination page={page} perPage={PER_PAGE} total={total} onPage={setPage} />

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
            <span className="field-heading">
              简介
              {form.id && canEdit && (
                <button
                  type="button"
                  className="muted-action"
                  disabled={generatingMetadataId !== null}
                  onClick={() => handleGenerateMetadata({ id: form.id!, description: form.description })}
                >
                  {generatingMetadataId === form.id ? <Loader size={13} className="spin" /> : <Sparkles size={13} />}
                  AI补全
                </button>
              )}
            </span>
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
          <div className="form-split">
            <label>
              地区
              <input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })} placeholder="中国大陆 / 美国 / 日本" />
            </label>
            <label>
              年份
              <input
                type="number"
                min={0}
                value={form.release_year || ''}
                onChange={e => setForm({ ...form, release_year: Number(e.target.value) || 0 })}
                placeholder="2024"
              />
            </label>
          </div>
          <div className="form-split">
            <label>
              语言
              <input value={form.language} onChange={e => setForm({ ...form, language: e.target.value })} placeholder="普通话 / 英语" />
            </label>
            <label>
              类型
              <input value={joinCatalogList(form.genres)} onChange={e => setForm({ ...form, genres: parseCatalogList(e.target.value) })} placeholder="动作、喜剧、动画" />
            </label>
          </div>
          <label>
            导演
            <input value={joinCatalogList(form.directors)} onChange={e => setForm({ ...form, directors: parseCatalogList(e.target.value) })} placeholder="多位导演用顿号分隔" />
          </label>
          <label>
            演员
            <input value={joinCatalogList(form.actors)} onChange={e => setForm({ ...form, actors: parseCatalogList(e.target.value) })} placeholder="多位演员用顿号分隔" />
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
                accept="video/mp4,video/x-matroska,.mkv"
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
              <input ref={mp4Ref} type="file" accept="video/mp4,video/x-matroska,.mkv" disabled={uploading} />
              <button type="button" disabled={uploading} onClick={() => handleUploadMp4(form.id!)}>
                <Loader size={13} className={uploading ? 'spin' : undefined} style={{ display: uploading ? undefined : 'none' }} />
                {!uploading && <CloudUpload size={13} />}
                {uploading
                  ? (uploadProgress !== null && uploadProgress < 99 ? `上传 ${uploadProgress}%` : '服务器处理中…')
                  : '上传 MP4'}
              </button>
              {uploading && (
                <button type="button" className="danger" onClick={() => cancelUploadIfActive(form.id!)}>
                  <XCircle size={13} /> 取消上传
                </button>
              )}
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

      {transcodeDialog && (
        <div className="confirm-backdrop">
          <div className="confirm-dialog" role="dialog" aria-modal="true" aria-label="选择转码分辨率">
            <div>
              <h3>选择转码分辨率</h3>
              <p>{transcodeDialog.video.title}</p>
            </div>
            <div className="transcode-quality-grid">
              {TRANSCODE_QUALITIES.map(quality => {
                const done = transcodeDialog.video.transcoded_qualities?.includes(quality) ?? false
                const available = transcodeDialog.video.available_transcode_qualities
                const supported = !available?.length || available.includes(quality)
                const currentTask = taskStatus[transcodeDialog.video.id]
                const qualityStatus = currentTask?.quality_statuses?.[quality]
                const displayStatus = effectiveQualityStatus(qualityStatus, done)
                const qualityMessage = currentTask?.quality_messages?.[quality] ?? ''
                const qualityProgress = currentTask?.quality_progress?.[quality]
                const active = isActiveTranscodeStatus(qualityStatus)
                const failed = displayStatus === 'failed'
                const canceled = displayStatus === 'canceled'
                const stateLabel = failed
                  ? (qualityMessage || '失败')
                  : canceled
                    ? '已取消'
                  : active
                    ? transcodeStateLabel(qualityStatus, qualityMessage, qualityProgress)
                    : done ? '已转' : supported ? '待转' : '不支持'
                const badgeLabel = failed
                  ? '失败'
                  : canceled
                    ? '已取消'
                  : active
                    ? '进行中'
                    : done ? '已转' : supported ? '待转' : '不支持'
                const showDetail = stateLabel !== badgeLabel
                return (
                  <label key={quality} className={`transcode-quality-option ${done ? 'is-transcoded' : ''} ${failed ? 'is-failed' : ''} ${supported ? '' : 'is-unsupported'}`}>
                    <input
                      type="checkbox"
                      checked={transcodeDialog.selected.includes(quality)}
                      onChange={() => toggleTranscodeQuality(quality)}
                      disabled={!supported || active}
                    />
                    <span className="transcode-quality-name">{quality}</span>
                    <span className={`transcode-quality-state ${failed ? 'failed' : done ? 'done' : canceled ? 'unsupported' : supported ? 'pending' : 'unsupported'}`}>
                      {badgeLabel}
                    </span>
                    {showDetail && <span className="transcode-quality-detail">{stateLabel}</span>}
                    {failed && supported && (
                      <button
                        type="button"
                        className="transcode-quality-retry"
                        disabled={transcoding}
                        onClick={e => {
                          e.preventDefault()
                          e.stopPropagation()
                          handleTranscode(transcodeDialog.video.id, [quality])
                        }}
                      >
                        重试
                      </button>
                    )}
                  </label>
                )
              })}
            </div>
            <div className="confirm-actions">
              <button
                className="ghost-button"
                disabled={transcoding}
                type="button"
                onClick={() => setTranscodeDialog(null)}
              >
                取消
              </button>
              <button
                className="primary-button"
                disabled={transcoding || transcodeDialog.selected.length === 0}
                type="button"
                onClick={() => handleTranscode(transcodeDialog.video.id, transcodeDialog.selected)}
              >
                {transcoding && <Loader size={14} className="spin" />}
                提交转码
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
