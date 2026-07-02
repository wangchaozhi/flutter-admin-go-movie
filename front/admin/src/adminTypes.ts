export type Entity = 'dashboard' | 'users' | 'roles' | 'menus' | 'videos' | 'series' | 'video-transcodes' | 'video-extracts' | 'categories' | 'app-users' | 'invite-codes' | 'payments' | 'comments' | 'audit-logs'

export type SeriesStatus = 'ongoing' | 'completed' | 'offline'

export type Series = {
  id: number
  title: string
  description: string
  cover_key: string
  category_id: number
  region: string
  release_year: number
  genres: string[]
  is_vip: boolean
  status: SeriesStatus
  created_at: string
  updated_at: string
  episode_count?: number
  cover_url?: string
  category_name?: string
}

export type SeriesForm = {
  id?: number
  title: string
  description: string
  category_id: number
  region: string
  release_year: number
  genres: string[]
  is_vip: boolean
  status: SeriesStatus
}

export type SeriesEpisode = {
  id: number
  title: string
  episode_number: number
  duration: number
  status: VideoStatus
  is_vip: boolean
  is_free: boolean
  cover_url: string
}

export type InviteCode = {
  id: number
  code: string
  max_uses: number
  used_count: number
  status: 'active' | 'disabled'
  note: string
  created_by: string
  expires_at: string | null
  created_at: string
  updated_at: string
}

export type AuditLog = {
  id: number
  request_id: string
  username: string
  method: string
  path: string
  status: number
  ip: string
  created_at: string
}

export type AdminComment = {
  id: number
  video_id: number
  user_id: number
  content: string
  rating: number
  created_at: string
  nickname: string
  username: string
  video_title: string
}

export type DashboardStats = {
  videos: { total: number; vip: number; free: number; by_status: Array<{ key: string; count: number }> }
  categories: { total: number }
  users: { total: number; vip: number; banned: number }
  orders: { total: number; by_status: Array<{ key: string; count: number }> }
  revenue: Array<{ currency: string; amount_cents: number }>
  revenue_trend: { currency: string; points: Array<{ date: string; amount_cents: number; orders: number }> }
  top_videos: Array<{ video_id: number; title: string; plays: number }>
}

export type VideoStatus = 'uploading' | 'extracting' | 'uploaded' | 'transcoding' | 'ready' | 'failed' | 'offline'

export type Category = {
  id: number
  name: string
  sort_order: number
  created_at: string
}

export type CategoryForm = {
  id?: number
  name: string
  sort_order: number
}

export type Video = {
  id: number
  title: string
  description: string
  category_id: number
  actors: string[]
  directors: string[]
  genres: string[]
  region: string
  release_year: number
  language: string
  cover_key: string
  original_key: string
  hls_master_key: string
  duration: number
  size: number
  source_width: number
  source_height: number
  audio_track_count: number
  subtitle_track_count: number
  media_tracks_scanned: boolean
  status: VideoStatus
  is_vip: boolean
  is_free: boolean
  series_id: number
  episode_number: number
  transcoded_qualities?: string[]
  available_transcode_qualities?: string[]
  transcoding?: boolean
  created_at: string
  updated_at: string
}

export type VideoAIMetadata = {
  video_id: number
  provider: string
  model: string
  status: string
  synopsis: string
  highlights: string[]
  tags: string[]
  generated_at?: string | null
  created_at: string
  updated_at: string
}

export type VideoForm = {
  id?: number
  title: string
  description: string
  category_id: number
  actors: string[]
  directors: string[]
  genres: string[]
  region: string
  release_year: number
  language: string
  is_vip: boolean
  is_free: boolean
}

export type TranscodeTaskStatus = 'queued' | 'pending' | 'processing' | 'success' | 'failed' | 'canceled'

export type TranscodeTask = {
  id: number
  video_id: number
  batch_id: number
  quality: string
  previous_status: string
  status: TranscodeTaskStatus
  status_message: string
  progress: number
  attempt: number
  error_message: string
  quality_statuses?: Record<string, TranscodeTaskStatus>
  quality_messages?: Record<string, string>
  quality_progress?: Record<string, number>
  started_at: string | null
  finished_at: string | null
  created_at: string
}

// One per-quality transcode entry shown under a video row, with whether the
// quality is currently present in the playable master playlist.
export type VideoQualityTask = TranscodeTask & { transcoded?: boolean }

export type TranscodeHistoryItem = TranscodeTask & {
  video_title: string
}

export type ExtractTaskStatus = 'processing' | 'success' | 'failed' | 'canceled'

export type ExtractHistoryItem = {
  id: number
  video_id: number
  video_title: string
  source_key: string
  status: ExtractTaskStatus
  status_message: string
  audio_count: number
  subtitle_count: number
  ready_count: number
  failed_count: number
  error_message: string
  started_at: string | null
  finished_at: string | null
  created_at: string
}

export type User = {
  id: number
  username: string
  nickname: string
  roleIds: number[]
}

export type Role = {
  id: number
  name: string
  key: string
  menuIds: number[]
}

export type Menu = {
  id: number
  name: string
  path: string
  parentId: number
  type: 'menu' | 'button'
  permission: string
  icon: string
  sortOrder: number
}

export type ApiResponse<T> = {
  code: number
  msg: string
  data?: T
}

export type Paged<T> = {
  items: T[]
  total: number
  page: number
  per_page: number
}

export type LoginResponse = {
  token: string
  username: string
  client: string
  menuPaths?: string[]
  permissions?: string[]
  theme?: ThemeMode
  avatarUrl?: string
  thumbnailUrl?: string
}

export type AdminSession = LoginResponse
export type ThemeMode = 'system' | 'light' | 'dark'
export type Profile = {
  username: string
  menuPaths: string[]
  permissions: string[]
  theme: ThemeMode
  avatarUrl: string
  thumbnailUrl: string
}

export type AppUser = {
  id: number
  username: string
  nickname: string
  email: string
  status: 'active' | 'banned'
  vip_until: string | null
  created_at: string
  updated_at: string
}

export type AppUserForm = {
  id?: number
  username: string
  password: string
  nickname: string
  email: string
  status: 'active' | 'banned'
}

export type CurrencyCode = 'CNY' | 'USD' | 'EUR' | 'JPY' | 'HKD' | 'TWD' | 'GBP' | 'AUD' | 'CAD' | 'SGD'
export type ProductKind = 'vip' | 'video'
export type ProductStatus = 'active' | 'inactive'
export type OrderProvider = 'stripe' | 'paypal' | 'wechat' | 'alipay' | 'mock'
export type OrderStatus = 'pending' | 'paying' | 'paid' | 'failed' | 'cancelled' | 'refunded'

export type Product = {
  id: number
  code: string
  name: string
  description: string
  kind: ProductKind
  price_cents: number
  currency: CurrencyCode
  duration_days: number
  video_id?: number | null
  status: ProductStatus
}

export type Order = {
  id: number
  order_no: string
  user_id: number
  product_id: number
  provider: OrderProvider
  status: OrderStatus
  amount_cents: number
  currency: CurrencyCode
  provider_order_id: string
  provider_payment_id: string
  checkout_url: string
  paid_at: string | null
  expires_at: string | null
  created_at: string
  updated_at: string
  product?: Product
  user?: AppUser
}

export type UserForm = Omit<User, 'id'> & { id?: number; password: string }
export type RoleForm = Omit<Role, 'id'> & { id?: number }
export type MenuForm = Omit<Menu, 'id'> & { id?: number }
export type ConfirmDialogState = {
  title: string
  message: string
  confirmLabel: string
  cancelLabel?: string
  variant?: 'primary' | 'danger'
  onConfirm: () => void
}
