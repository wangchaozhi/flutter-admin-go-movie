import type { LucideIcon } from 'lucide-react'
import {
  AudioLines,
  BadgeCheck,
  Clapperboard,
  CreditCard,
  FolderOpen,
  History,
  LayoutDashboard,
  List,
  Menu,
  Pencil,
  Plus,
  RotateCcw,
  Settings,
  Shield,
  Smartphone,
  Trash2,
  UserCog,
  Users,
} from 'lucide-react'

export const iconRegistry: Record<string, LucideIcon> = {
  AudioLines,
  BadgeCheck,
  Clapperboard,
  CreditCard,
  FolderOpen,
  History,
  LayoutDashboard,
  List,
  Menu,
  Pencil,
  Plus,
  RotateCcw,
  Settings,
  Shield,
  Smartphone,
  Trash2,
  UserCog,
  Users,
}

export const menuIconOptions = [
  { value: 'Settings', label: 'Settings - 系统' },
  { value: 'Users', label: 'Users - 用户' },
  { value: 'Shield', label: 'Shield - 角色/权限' },
  { value: 'Menu', label: 'Menu - 菜单' },
  { value: 'Smartphone', label: 'Smartphone - App 用户' },
  { value: 'Clapperboard', label: 'Clapperboard - 视频' },
  { value: 'FolderOpen', label: 'FolderOpen - 类别' },
  { value: 'History', label: 'History - 历史' },
  { value: 'AudioLines', label: 'AudioLines - 音轨/提取' },
  { value: 'CreditCard', label: 'CreditCard - 支付' },
  { value: 'LayoutDashboard', label: 'LayoutDashboard - 仪表盘' },
  { value: 'List', label: 'List - 列表' },
  { value: 'Plus', label: 'Plus - 新增' },
  { value: 'Pencil', label: 'Pencil - 编辑' },
  { value: 'Trash2', label: 'Trash2 - 删除' },
  { value: 'RotateCcw', label: 'RotateCcw - 重试/退款' },
  { value: 'BadgeCheck', label: 'BadgeCheck - 权益/确认' },
]

export function resolveIcon(code?: string, fallback: LucideIcon = List) {
  const key = code?.trim()
  return key && iconRegistry[key] ? iconRegistry[key] : fallback
}
