export type Entity = 'users' | 'roles' | 'menus' | 'videos' | 'video-transcodes' | 'categories' | 'app-users' | 'payments'

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
  cover_key: string
  original_key: string
  hls_master_key: string
  duration: number
  size: number
  source_width: number
  source_height: number
  status: VideoStatus
  is_vip: boolean
  is_free: boolean
  transcoded_qualities?: string[]
  available_transcode_qualities?: string[]
  created_at: string
  updated_at: string
}

export type VideoForm = {
  id?: number
  title: string
  description: string
  category_id: number
  is_vip: boolean
  is_free: boolean
}

export type TranscodeTaskStatus = 'queued' | 'pending' | 'processing' | 'success' | 'failed'

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
export type OrderProvider = 'stripe' | 'paypal' | 'mock'
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
  onConfirm: () => void
}
