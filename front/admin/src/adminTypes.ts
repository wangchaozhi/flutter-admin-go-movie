export type Entity = 'users' | 'roles' | 'menus' | 'videos' | 'categories' | 'app-users'

export type VideoStatus = 'uploading' | 'uploaded' | 'transcoding' | 'ready' | 'failed' | 'offline'

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
  status: VideoStatus
  is_vip: boolean
  is_free: boolean
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

export type TranscodeTask = {
  id: number
  video_id: number
  status: 'pending' | 'processing' | 'success' | 'failed'
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

export type UserForm = Omit<User, 'id'> & { id?: number; password: string }
export type RoleForm = Omit<Role, 'id'> & { id?: number }
export type MenuForm = Omit<Menu, 'id'> & { id?: number }
export type ConfirmDialogState = {
  title: string
  message: string
  confirmLabel: string
  onConfirm: () => void
}
