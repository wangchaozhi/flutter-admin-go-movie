import { useEffect, useMemo, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import {
  BadgeCheck,
  ChevronRight,
  KeyRound,
  LogOut,
  Monitor,
  Moon,
  ImageUp,
  PanelLeft,
  RefreshCw,
  Sun,
} from 'lucide-react'
import './App.css'

import type {
  AdminSession,
  ApiResponse,
  ConfirmDialogState,
  Entity,
  LoginResponse,
  Menu,
  MenuForm,
  Profile,
  Role,
  RoleForm,
  ThemeMode,
  User,
  UserForm,
} from './adminTypes'
import { ConfirmDialog } from './components/confirm'
import { AppUserManagementSection } from './features/appUsers'
import { AuditLogsSection } from './features/audit'
import { CategoryManagementSection } from './features/categories'
import { CommentModerationSection } from './features/comments'
import { DashboardSection } from './features/dashboard'
import { InviteCodeManagementSection } from './features/inviteCodes'
import { MenuManagementSection, buildMenuTree } from './features/menus'
import type { MenuNodeType } from './features/menus'
import { PaymentManagementSection } from './features/payments'
import { RoleManagementSection } from './features/roles'
import { SeriesManagementSection } from './features/series'
import { UserManagementSection } from './features/users'
import { useI18n, LanguageSwitcher } from './i18n'
import {
  VideoManagementSection,
  VideoTranscodeHistorySection,
  VideoExtractHistorySection,
} from './features/videos'
import { resolveIcon } from './iconRegistry'

const emptyUser: UserForm = {
  username: '',
  nickname: '',
  password: '',
  roleIds: [],
}

const emptyRole: RoleForm = {
  name: '',
  key: '',
  menuIds: [],
}

const emptyMenu: MenuForm = {
  name: '',
  path: '',
  parentId: 0,
  type: 'menu',
  permission: '',
  icon: '',
  sortOrder: 0,
}

type ChildNavItem = {
  key: Entity
  label: string
  iconCode: string
  path: string
  sortOrder?: number
}

type NavItem = {
  key: string
  label: string
  iconCode: string
  path: string
  sortOrder?: number
  children?: ChildNavItem[]
}

const tabs: NavItem[] = [
  {
    key: 'system',
    label: '系统管理',
    iconCode: 'Settings',
    path: '/system',
    children: [
      { key: 'users', label: '管理员', iconCode: 'Users', path: '/system/user' },
      { key: 'roles', label: '角色', iconCode: 'Shield', path: '/system/role' },
      { key: 'menus', label: '菜单', iconCode: 'Menu', path: '/system/menu' },
    ],
  },
  {
    key: 'app',
    label: 'App 管理',
    iconCode: 'Smartphone',
    path: '/app',
    children: [
      { key: 'app-users', label: '用户', iconCode: 'Users', path: '/app-users' },
      { key: 'invite-codes', label: '邀请码', iconCode: 'Ticket', path: '/invite-codes' },
    ],
  },
  {
    key: 'video',
    label: '视频管理',
    iconCode: 'Clapperboard',
    path: '/videos',
    children: [
      { key: 'videos', label: '视频列表', iconCode: 'Clapperboard', path: '/videos' },
      { key: 'series', label: '剧集', iconCode: 'Layers', path: '/videos/series' },
      { key: 'categories', label: '类别', iconCode: 'FolderOpen', path: '/categories' },
      { key: 'video-transcodes', label: '转码历史', iconCode: 'History', path: '/videos/transcodes' },
      { key: 'video-extracts', label: '提取历史', iconCode: 'AudioLines', path: '/videos/extracts' },
    ],
  },
  { key: 'payments', label: '支付', iconCode: 'CreditCard', path: '/payments' },
]

// Page header (eyebrow/title/subtitle) is resolved from the i18n dictionary by
// entity key so it follows the active language. See src/i18n/messages.ts.
function getPageHeader(
  entity: Entity,
  t: (key: string, vars?: Record<string, string | number>) => string,
) {
  return {
    eyebrow: t(`headers.${entity}.eyebrow`),
    title: t(`headers.${entity}.title`),
    subtitle: t(`headers.${entity}.subtitle`),
  }
}

const routeMetaByPath = new Map<string, { key: Entity; label: string; iconCode: string }>([
  ['/dashboard', { key: 'dashboard', label: '仪表盘', iconCode: 'LayoutDashboard' }],
  ['/system/user', { key: 'users', label: '管理员', iconCode: 'Users' }],
  ['/system/role', { key: 'roles', label: '角色', iconCode: 'Shield' }],
  ['/system/menu', { key: 'menus', label: '菜单', iconCode: 'Menu' }],
  ['/app-users', { key: 'app-users', label: '用户', iconCode: 'Users' }],
  ['/invite-codes', { key: 'invite-codes', label: '邀请码', iconCode: 'Ticket' }],
  ['/videos', { key: 'videos', label: '视频列表', iconCode: 'Clapperboard' }],
  ['/videos/series', { key: 'series', label: '剧集', iconCode: 'Layers' }],
  ['/categories', { key: 'categories', label: '类别', iconCode: 'FolderOpen' }],
  ['/videos/transcodes', { key: 'video-transcodes', label: '转码历史', iconCode: 'History' }],
  ['/videos/extracts', { key: 'video-extracts', label: '提取历史', iconCode: 'AudioLines' }],
  ['/payments', { key: 'payments', label: '支付', iconCode: 'CreditCard' }],
  ['/comments', { key: 'comments', label: '评论', iconCode: 'MessageSquare' }],
  ['/audit-logs', { key: 'audit-logs', label: '审计日志', iconCode: 'ScrollText' }],
])

const legacyMenuNames = new Set([
  'system',
  'user',
  'role',
  'menu',
  'app-users',
  'videos',
  'categories',
  'payments',
  'video:transcode-history',
  'video:extract-history',
])

const groupLabelByPath = new Map([
  ['/system', '系统管理'],
  ['/app', 'App 管理'],
  ['/videos', '视频管理'],
])

const entityKeys = new Set<Entity>([
  'dashboard',
  'users',
  'roles',
  'menus',
  'videos',
  'series',
  'video-transcodes',
  'video-extracts',
  'categories',
  'app-users',
  'invite-codes',
  'payments',
  'comments',
  'audit-logs',
])

function isEntityKey(key: string): key is Entity {
  return entityKeys.has(key as Entity)
}

function buildVisibleNavigation(menuTree: MenuNodeType[], menuPaths: Set<string>) {
  const items = menuTree
    .map((node) => buildNavItem(node, menuPaths))
    .filter((item): item is NavItem => Boolean(item))
    .sort(compareNavItems)
  return items.length > 0 ? items : buildFallbackNavigation(menuPaths)
}

function buildNavItem(node: MenuNodeType, menuPaths: Set<string>): NavItem | null {
  const meta = routeMetaByPath.get(node.path)
  const visible = isPathVisible(node.path, menuPaths, false)
  const children = [
    ...(node.children.length > 0 && meta && visible
      ? [{
          key: meta.key,
          label: meta.label,
          iconCode: node.icon || meta.iconCode,
          path: node.path,
          sortOrder: 0,
        }]
      : []),
    ...node.children.flatMap((child) => collectNavLinks(child, menuPaths, visible)),
  ].sort(compareChildNavItems)

  if (children.length > 0) {
    return {
      key: `menu:${node.id}`,
      label: getGroupLabel(node, meta?.label),
      iconCode: node.icon || meta?.iconCode || children[0]?.iconCode || 'List',
      path: node.path,
      sortOrder: node.sortOrder,
      children,
    }
  }

  if (!meta || !visible) return null
  return {
    key: meta.key,
    label: getLinkLabel(node, meta.label),
    iconCode: node.icon || meta.iconCode,
    path: node.path,
    sortOrder: node.sortOrder,
  }
}

function collectNavLinks(
  node: MenuNodeType,
  menuPaths: Set<string>,
  inheritedVisible: boolean,
): ChildNavItem[] {
  const meta = routeMetaByPath.get(node.path)
  const visible = isPathVisible(node.path, menuPaths, inheritedVisible)
  const self = meta && visible
    ? [{
        key: meta.key,
        label: getLinkLabel(node, meta.label),
        iconCode: node.icon || meta.iconCode,
        path: node.path,
        sortOrder: node.sortOrder,
      }]
    : []
  return [
    ...self,
    ...node.children.flatMap((child) => collectNavLinks(child, menuPaths, visible)),
  ].sort(compareChildNavItems)
}

function buildFallbackNavigation(menuPaths: Set<string>) {
  return tabs
    .map((tab) => {
      if (!tab.children) return tab
      const children = menuPaths.size === 0 || menuPaths.has(tab.path)
        ? tab.children
        : tab.children.filter((child) => menuPaths.has(child.path))
      return { ...tab, children }
    })
    .filter((tab) => {
      if (menuPaths.size === 0) return true
      if (tab.children) return tab.children.length > 0
      return menuPaths.has(tab.path)
    })
}

function isPathVisible(path: string, menuPaths: Set<string>, inheritedVisible: boolean) {
  return inheritedVisible || menuPaths.size === 0 || (path !== '' && menuPaths.has(path))
}

function getGroupLabel(menu: Menu, fallback = '菜单') {
  const name = menu.name.trim()
  if (name && !legacyMenuNames.has(name)) return name
  return groupLabelByPath.get(menu.path) ?? fallback
}

function getLinkLabel(menu: Menu, fallback: string) {
  const name = menu.name.trim()
  if (name && !legacyMenuNames.has(name)) return name
  return fallback
}

function compareNavItems(left: NavItem, right: NavItem) {
  return (left.sortOrder ?? 0) - (right.sortOrder ?? 0) || left.key.localeCompare(right.key)
}

function compareChildNavItems(left: ChildNavItem, right: ChildNavItem) {
  return (left.sortOrder ?? 0) - (right.sortOrder ?? 0) || left.key.localeCompare(right.key)
}

const adminRememberKey = 'admin.remember'
const adminUsernameKey = 'admin.username'
const adminPasswordKey = 'admin.password'
const adminSessionKey = 'admin.session'
const adminThemeKey = 'admin.theme'
const themeOrder: ThemeMode[] = ['system', 'light', 'dark']

function getStoredTheme(): ThemeMode {
  const value = localStorage.getItem(adminThemeKey)
  return value === 'light' || value === 'dark' || value === 'system' ? value : 'system'
}

function getThemeIcon(theme: ThemeMode) {
  if (theme === 'light') return Sun
  if (theme === 'dark') return Moon
  return Monitor
}

function nextTheme(theme: ThemeMode): ThemeMode {
  return themeOrder[(themeOrder.indexOf(theme) + 1) % themeOrder.length]
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    ...authHeaders(),
  }
  if (!(init?.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }
  const res = await fetch(url, {
    ...init,
    headers: {
      ...headers,
      ...(init?.headers as Record<string, string> | undefined),
    },
  })
  const body = (await res.json()) as ApiResponse<T>
  if (!res.ok || body.code !== 0) {
    throw new Error(body.msg || '请求失败')
  }
  return body.data as T
}

function authHeaders(): Record<string, string> {
  const rawSession = localStorage.getItem(adminSessionKey)
  let session: AdminSession | null = null
  try {
    session = rawSession ? (JSON.parse(rawSession) as AdminSession) : null
  } catch {
    localStorage.removeItem(adminSessionKey)
  }
  const authHeaders: Record<string, string> = session?.token
    ? { Authorization: `Bearer ${session.token}` }
    : {}
  return authHeaders
}

async function fetchAssetObjectURL(url: string): Promise<string> {
  const res = await fetch(url, { headers: authHeaders() })
  if (!res.ok) {
    throw new Error('加载头像失败')
  }
  return URL.createObjectURL(await res.blob())
}

function App() {
  const [theme, setTheme] = useState<ThemeMode>(getStoredTheme)
  const [session, setSession] = useState<AdminSession | null>(() => {
    const raw = localStorage.getItem(adminSessionKey)
    if (!raw) return null
    try {
      const stored = JSON.parse(raw) as AdminSession
      if (!stored.permissions || !stored.menuPaths) {
        localStorage.removeItem(adminSessionKey)
        return null
      }
      return stored
    } catch {
      localStorage.removeItem(adminSessionKey)
      return null
    }
  })

  function handleLoggedIn(nextSession: AdminSession) {
    const nextThemeValue = nextSession.theme ?? getStoredTheme()
    localStorage.setItem(adminSessionKey, JSON.stringify(nextSession))
    setTheme(nextThemeValue)
    setSession(nextSession)
  }

  function handleLogout() {
    localStorage.removeItem(adminSessionKey)
    setSession(null)
  }

  function handleThemeChange() {
    const next = nextTheme(theme)
    setTheme(next)
    if (!session) return
    const nextSession = { ...session, theme: next }
    localStorage.setItem(adminSessionKey, JSON.stringify(nextSession))
    setSession(nextSession)
    void request('/api/admin/profile/theme', {
      method: 'PUT',
      body: JSON.stringify({ theme: next }),
    })
  }

  function handleSessionChange(nextSession: AdminSession) {
    localStorage.setItem(adminSessionKey, JSON.stringify(nextSession))
    setSession(nextSession)
  }

  useEffect(() => {
    localStorage.setItem(adminThemeKey, theme)
    document.documentElement.dataset.theme = theme
  }, [theme])

  useEffect(() => {
    if (session?.theme) {
      setTheme(session.theme)
    }
    // re-sync theme only when the signed-in user changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session?.username])

  if (!session) {
    return <AdminLogin theme={theme} onThemeChange={handleThemeChange} onLoggedIn={handleLoggedIn} />
  }

  return (
    <AdminDashboard
      session={session}
      theme={theme}
      onSessionChange={handleSessionChange}
      onThemeChange={handleThemeChange}
      onLogout={handleLogout}
    />
  )
}

function AdminLogin({
  theme,
  onThemeChange,
  onLoggedIn,
}: {
  theme: ThemeMode
  onThemeChange: () => void
  onLoggedIn: (session: AdminSession) => void
}) {
  const { t } = useI18n()
  const [username, setUsername] = useState(() => localStorage.getItem(adminUsernameKey) ?? 'admin')
  const [password, setPassword] = useState(() => localStorage.getItem(adminPasswordKey) ?? '')
  const [remember, setRemember] = useState(() => localStorage.getItem(adminRememberKey) === 'true')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function login(event: FormEvent) {
    event.preventDefault()
    if (!username.trim() || !password) {
      setError(t('login.errCredentials'))
      return
    }
    setLoading(true)
    setError('')
    try {
      const data = await request<LoginResponse>('/api/admin/login', {
        method: 'POST',
        body: JSON.stringify({ username: username.trim(), password }),
      })
      if (remember) {
        localStorage.setItem(adminRememberKey, 'true')
        localStorage.setItem(adminUsernameKey, username.trim())
        localStorage.setItem(adminPasswordKey, password)
      } else {
        localStorage.removeItem(adminRememberKey)
        localStorage.removeItem(adminUsernameKey)
        localStorage.removeItem(adminPasswordKey)
      }
      onLoggedIn(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('login.errFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-shell">
      <div className="login-toolbar">
        <LanguageSwitcher />
        <ThemeButton theme={theme} onThemeChange={onThemeChange} />
      </div>
      <form className="login-card" onSubmit={login}>
        <span className="brand-mark">
          <PanelLeft size={18} strokeWidth={2.2} />
        </span>
        <div className="login-heading">
          <p className="eyebrow">{t('login.eyebrow')}</p>
          <h1>{t('login.title')}</h1>
          <p>{t('login.subtitle')}</p>
        </div>
        <label>
          {t('login.username')}
          <input value={username} onChange={(event) => setUsername(event.target.value)} />
        </label>
        <label>
          {t('login.password')}
          <input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        <label className="remember-row">
          <input
            checked={remember}
            type="checkbox"
            onChange={(event) => setRemember(event.target.checked)}
          />
          <span>{t('login.remember')}</span>
        </label>
        {error && <span className="status error">{error}</span>}
        <button className="primary-button" disabled={loading} type="submit">
          <KeyRound size={15} />
          {loading ? t('login.signingIn') : t('login.signIn')}
        </button>
      </form>
    </main>
  )
}

function AdminDashboard({
  session,
  theme,
  onSessionChange,
  onThemeChange,
  onLogout,
}: {
  session: AdminSession
  theme: ThemeMode
  onSessionChange: (session: AdminSession) => void
  onThemeChange: () => void
  onLogout: () => void
}) {
  const { t } = useI18n()
  const [active, setActive] = useState<Entity>('users')
  const [users, setUsers] = useState<User[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [menus, setMenus] = useState<Menu[]>([])
  const [userForm, setUserForm] = useState<UserForm>(emptyUser)
  const [roleForm, setRoleForm] = useState<RoleForm>(emptyRole)
  const [menuForm, setMenuForm] = useState<MenuForm>(emptyMenu)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState('正在加载管理数据')
  const [error, setError] = useState('')
  const [avatarPreview, setAvatarPreview] = useState('')
  const [avatarRefreshKey, setAvatarRefreshKey] = useState(0)
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null)
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const [openNavGroups, setOpenNavGroups] = useState<Record<string, boolean>>({})
  const userMenuRef = useRef<HTMLDivElement | null>(null)

  const roleNameByID = useMemo(
    () => new Map(roles.map((role) => [role.id, role.name])),
    [roles],
  )
  const menuNameByID = useMemo(
    () => new Map(menus.map((menu) => [menu.id, menu.name])),
    [menus],
  )
  const pageMenus = useMemo(() => menus.filter((menu) => menu.type !== 'button'), [menus])
  const buttonMenus = useMemo(() => menus.filter((menu) => menu.type === 'button'), [menus])
  const menuTree = useMemo(() => buildMenuTree(menus), [menus])
  const navMenuTree = useMemo(() => buildMenuTree(pageMenus), [pageMenus])
  const permissions = useMemo(() => new Set(session.permissions ?? []), [session.permissions])
  const menuPaths = useMemo(() => new Set(session.menuPaths ?? []), [session.menuPaths])
  const visibleTabs = useMemo(
    () => buildVisibleNavigation(navMenuTree, menuPaths),
    [navMenuTree, menuPaths],
  )
  const flatVisibleTabs = useMemo<ChildNavItem[]>(
    () =>
      visibleTabs.flatMap((tab) => {
        if (tab.children?.length) return tab.children
        return isEntityKey(tab.key)
          ? [{ key: tab.key, label: tab.label, iconCode: tab.iconCode, path: tab.path }]
          : []
      }),
    [visibleTabs],
  )
  const activeHeader = getPageHeader(active, t)
  const can = (permission: string) => permissions.has(permission)

  // Nav labels follow the active language for known routes/entities, falling
  // back to the (DB-supplied) label for any custom menu without a translation.
  const navGroupLabel = (path: string, fallback: string) => {
    const key = `navGroup.${path}`
    const value = t(key)
    return value === key ? fallback : value
  }
  const navEntityLabel = (entity: string, fallback: string) => {
    const key = `navEntity.${entity}`
    const value = t(key)
    return value === key ? fallback : value
  }

  function toggleNavGroup(key: string) {
    setOpenNavGroups((current) => ({ ...current, [key]: !(current[key] ?? true) }))
  }

  async function loadAll() {
    setLoading(true)
    setError('')
    try {
      const [nextUsers, nextRoles, nextMenus, nextProfile] = await Promise.all([
        request<User[]>('/api/admin/users'),
        request<Role[]>('/api/admin/roles'),
        request<Menu[]>('/api/admin/menus'),
        request<Profile>('/api/admin/profile'),
      ])
      setUsers(nextUsers ?? [])
      setRoles(nextRoles ?? [])
      setMenus(nextMenus ?? [])
      onSessionChange({
        ...session,
        theme: nextProfile.theme,
        menuPaths: nextProfile.menuPaths,
        permissions: nextProfile.permissions,
        avatarUrl: nextProfile.avatarUrl,
        thumbnailUrl: nextProfile.thumbnailUrl,
      })
      setNotice('数据已同步')
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败')
      setNotice('数据加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadAll()
    // mount-only load; `loadAll` is recreated each render
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (flatVisibleTabs.length > 0 && !flatVisibleTabs.some((tab) => tab.key === active)) {
      setActive(flatVisibleTabs[0].key)
    }
  }, [active, flatVisibleTabs])

  useEffect(() => {
    if (!userMenuOpen) return

    function closeUserMenu(event: Event) {
      const menu = userMenuRef.current
      if (!menu) return

      const path = event.composedPath()
      if (!path.includes(menu)) {
        setUserMenuOpen(false)
      }
    }

    document.addEventListener('mousedown', closeUserMenu, true)
    document.addEventListener('touchstart', closeUserMenu, true)
    return () => {
      document.removeEventListener('mousedown', closeUserMenu, true)
      document.removeEventListener('touchstart', closeUserMenu, true)
    }
  }, [userMenuOpen])

  useEffect(() => {
    if (!session.thumbnailUrl) {
      setAvatarPreview('')
      return
    }
    let revoked = false
    const separator = session.thumbnailUrl.includes('?') ? '&' : '?'
    const thumbnailUrl = `${session.thumbnailUrl}${separator}v=${avatarRefreshKey}`
    void fetchAssetObjectURL(thumbnailUrl)
      .then((url) => {
        if (revoked) {
          URL.revokeObjectURL(url)
          return
        }
        setAvatarPreview(url)
      })
      .catch(() => setAvatarPreview(''))
    return () => {
      revoked = true
      setAvatarPreview((current) => {
        if (current) URL.revokeObjectURL(current)
        return ''
      })
    }
  }, [avatarRefreshKey, session.thumbnailUrl])

  async function saveUser(event: FormEvent) {
    event.preventDefault()
    if (!userForm.username.trim()) {
      setError('请输入用户名')
      return
    }
    if (!userForm.id && !userForm.password.trim()) {
      setError('新增用户需要设置密码')
      return
    }
    await saveRecord(
      'users',
      userForm.id,
      {
        username: userForm.username.trim(),
        nickname: userForm.nickname.trim(),
        password: userForm.password.trim(),
        roleIds: userForm.roleIds,
      },
      () => setUserForm(emptyUser),
    )
  }

  async function saveRole(event: FormEvent) {
    event.preventDefault()
    if (!roleForm.name.trim() || !roleForm.key.trim()) {
      setError('请输入角色名称和标识')
      return
    }
    await saveRecord(
      'roles',
      roleForm.id,
      {
        name: roleForm.name.trim(),
        key: roleForm.key.trim(),
        menuIds: roleForm.menuIds,
      },
      () => setRoleForm(emptyRole),
    )
  }

  async function saveMenu(event: FormEvent) {
    event.preventDefault()
    if (
      !menuForm.name.trim() ||
      (menuForm.type !== 'button' && !menuForm.path.trim()) ||
      (menuForm.type === 'button' && !menuForm.permission.trim())
    ) {
      setError('请输入菜单名称和路径')
      return
    }
    if (menuForm.id && menuForm.parentId === menuForm.id) {
      setError('上级菜单不能选择自己')
      return
    }
    await saveRecord(
      'menus',
      menuForm.id,
      {
        name: menuForm.name.trim(),
        path: menuForm.path.trim(),
        parentId: menuForm.parentId,
        type: menuForm.type,
        permission: menuForm.permission.trim(),
        icon: menuForm.icon.trim(),
        sortOrder: menuForm.sortOrder,
      },
      () => setMenuForm(emptyMenu),
    )
  }

  async function saveRecord(
    entity: Entity,
    id: number | undefined,
    payload: unknown,
    reset: () => void,
  ) {
    setSaving(true)
    setError('')
    try {
      await request(`/api/admin/${entity}${id ? `/${id}` : ''}`, {
        method: id ? 'PUT' : 'POST',
        body: JSON.stringify(payload),
      })
      reset()
      await loadAll()
      setNotice(id ? '修改已保存' : '新增成功')
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  function deleteRecord(entity: Entity, id: number) {
    setConfirmDialog({
      title: '确认删除',
      message: '删除后无法恢复，确定要删除这条数据吗？',
      confirmLabel: '删除',
      onConfirm: () => void performDeleteRecord(entity, id),
    })
  }

  async function performDeleteRecord(entity: Entity, id: number) {
    setConfirmDialog(null)
    setSaving(true)
    setError('')
    try {
      await request(`/api/admin/${entity}/${id}`, { method: 'DELETE' })
      await loadAll()
      setNotice('删除成功')
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败')
    } finally {
      setSaving(false)
    }
  }

  async function uploadAvatar(file: File | undefined) {
    if (!file) return
    setUserMenuOpen(false)
    setSaving(true)
    setError('')
    try {
      const form = new FormData()
      form.append('avatar', file)
      const profile = await request<Profile>('/api/admin/profile/avatar', {
        method: 'POST',
        body: form,
      })
      onSessionChange({
        ...session,
        theme: profile.theme,
        avatarUrl: profile.avatarUrl,
        thumbnailUrl: profile.thumbnailUrl,
      })
      setAvatarRefreshKey(Date.now())
      setNotice('头像已更新')
    } catch (err) {
      setError(err instanceof Error ? err.message : '头像上传失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <main className="admin-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">
            <PanelLeft size={18} strokeWidth={2.2} />
          </span>
          <div>
            <strong>Admin Go</strong>
            <small>{t('brand.subtitle')}</small>
          </div>
        </div>
        <nav className="nav-tabs" aria-label={t('brand.subtitle')}>
          {visibleTabs.map((tab) => {
            const Icon = resolveIcon(tab.iconCode)
            const children = tab.children ?? []
            const activeChild = children.some((child) => child.key === active)
            const isGroup = children.length > 0
            const groupOpen = isGroup ? openNavGroups[tab.key] ?? true : false
            const tabLabel = isGroup
              ? navGroupLabel(tab.path, tab.label)
              : isEntityKey(tab.key)
                ? navEntityLabel(tab.key, tab.label)
                : tab.label
            return (
              <div className="nav-group" key={tab.key}>
                <button
                  className={(isEntityKey(tab.key) && active === tab.key) || activeChild ? 'active' : ''}
                  type="button"
                  aria-expanded={isGroup ? groupOpen : undefined}
                  onClick={() => {
                    if (isGroup) {
                      toggleNavGroup(tab.key)
                      return
                    }
                    if (isEntityKey(tab.key)) {
                      setActive(tab.key)
                    }
                  }}
                >
                  <Icon size={16} />
                  <span>{tabLabel}</span>
                  <ChevronRight className={groupOpen ? 'nav-chevron open' : 'nav-chevron'} size={15} />
                </button>
                {children.length > 0 && groupOpen && (
                  <div className="nav-subtabs">
                    {children.map((child) => {
                      const ChildIcon = resolveIcon(child.iconCode)
                      return (
                        <button
                          className={active === child.key ? 'active' : ''}
                          key={child.key}
                          type="button"
                          onClick={() => setActive(child.key)}
                        >
                          <ChildIcon size={14} />
                          <span>{navEntityLabel(child.key, child.label)}</span>
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
            )
          })}
        </nav>
      </aside>

      <section className="workspace">
        <header className="toolbar">
          <div>
            <p className="eyebrow">{activeHeader.eyebrow}</p>
            <h1>{activeHeader.title}</h1>
            <p className="toolbar-subtitle">{activeHeader.subtitle}</p>
          </div>
          <div className="toolbar-actions">
            <button className="ghost-button" type="button" onClick={loadAll}>
              <RefreshCw size={15} />
              {t('common.refresh')}
            </button>
            <LanguageSwitcher />
            <ThemeButton theme={theme} onThemeChange={onThemeChange} />
            <div className="user-menu" ref={userMenuRef}>
              <button
                className="session-pill"
                type="button"
                aria-expanded={userMenuOpen}
                onClick={() => setUserMenuOpen((open) => !open)}
              >
                {avatarPreview ? (
                  <img alt={session.username} src={avatarPreview} />
                ) : (
                  <BadgeCheck size={14} />
                )}
                <span>{session.username}</span>
                <ChevronRight className={userMenuOpen ? 'menu-chevron open' : 'menu-chevron'} size={15} />
              </button>
              {userMenuOpen && (
                <div className="user-menu-popover">
                  <label className="user-menu-item">
                    <ImageUp size={15} />
                    {t('userMenu.changeAvatar')}
                    <input
                      accept="image/png,image/jpeg"
                      type="file"
                      onChange={(event) => {
                        void uploadAvatar(event.target.files?.[0])
                        event.target.value = ''
                      }}
                    />
                  </label>
                  <button className="user-menu-item danger" type="button" onClick={onLogout}>
                    <LogOut size={15} />
                    {t('userMenu.logout')}
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>

        <div className="status-row">
          <span className={error ? 'status error' : 'status'}>{error || notice}</span>
          {loading && <span className="status subtle">{t('common.loading')}</span>}
        </div>

        {active === 'dashboard' && (
          <DashboardSection token={session.token} />
        )}

        {active === 'users' && (
          <UserManagementSection
            users={users}
            roles={roles}
            roleNameByID={roleNameByID}
            userForm={userForm}
            saving={saving}
            can={can}
            onUserFormChange={setUserForm}
            onSaveUser={saveUser}
            onDeleteUser={(id) => deleteRecord('users', id)}
          />
        )}

        {active === 'roles' && (
          <RoleManagementSection
            roles={roles}
            pageMenus={pageMenus}
            buttonMenus={buttonMenus}
            menuNameByID={menuNameByID}
            roleForm={roleForm}
            saving={saving}
            can={can}
            onRoleFormChange={setRoleForm}
            onSaveRole={saveRole}
            onDeleteRole={(id) => deleteRecord('roles', id)}
          />
        )}

        {active === 'menus' && (
          <MenuManagementSection
            menus={menus}
            menuTree={menuTree}
            pageMenus={pageMenus}
            menuForm={menuForm}
            saving={saving}
            can={can}
            onMenuFormChange={setMenuForm}
            onSaveMenu={saveMenu}
            onDeleteMenu={(id) => deleteRecord('menus', id)}
          />
        )}

        {active === 'app-users' && (
          <AppUserManagementSection token={session.token} can={can} />
        )}

        {active === 'invite-codes' && (
          <InviteCodeManagementSection token={session.token} can={can} />
        )}

        {active === 'categories' && (
          <CategoryManagementSection token={session.token} can={can} />
        )}

        {active === 'videos' && (
          <VideoManagementSection token={session.token} can={can} />
        )}

        {active === 'series' && (
          <SeriesManagementSection token={session.token} can={can} />
        )}

        {active === 'video-transcodes' && (
          <VideoTranscodeHistorySection token={session.token} can={can} />
        )}

        {active === 'video-extracts' && (
          <VideoExtractHistorySection token={session.token} can={can} />
        )}

        {active === 'payments' && (
          <PaymentManagementSection token={session.token} can={can} />
        )}

        {active === 'comments' && (
          <CommentModerationSection token={session.token} can={can} />
        )}

        {active === 'audit-logs' && (
          <AuditLogsSection token={session.token} />
        )}
      </section>

      <ConfirmDialog
        state={confirmDialog}
        busy={saving}
        onCancel={() => setConfirmDialog(null)}
      />
    </main>
  )
}

function ThemeButton({
  theme,
  onThemeChange,
  className = '',
}: {
  theme: ThemeMode
  onThemeChange: () => void
  className?: string
}) {
  const { t } = useI18n()
  const Icon = getThemeIcon(theme)
  const label = t(`theme.${theme}`)
  return (
    <button
      className={`ghost-button theme-button ${className}`.trim()}
      type="button"
      title={`${t('theme.label')}: ${label}`}
      onClick={onThemeChange}
    >
      <Icon size={15} />
      <span>{label}</span>
    </button>
  )
}

export default App
