export type Entity = 'users' | 'roles' | 'menus'

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

export type UserForm = Omit<User, 'id'> & { id?: number; password: string }
export type RoleForm = Omit<Role, 'id'> & { id?: number }
export type MenuForm = Omit<Menu, 'id'> & { id?: number }
export type ConfirmDialogState = {
  title: string
  message: string
  confirmLabel: string
  onConfirm: () => void
}
