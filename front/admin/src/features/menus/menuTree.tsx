import { useState } from 'react'
import { ChevronRight } from 'lucide-react'

import type { Menu } from '../../adminTypes'
import { RowActions } from '../../components/shared'

export type MenuNodeType = Menu & { children: MenuNodeType[] }

export function MenuNode({
  node,
  onEdit,
  onDelete,
  canEdit,
  canDelete,
}: {
  node: MenuNodeType
  onEdit: (menu: Menu) => void
  onDelete: (id: number) => void
  canEdit: boolean
  canDelete: boolean
}) {
  const [expanded, setExpanded] = useState(false)
  const hasChildren = node.children.length > 0

  return (
    <div className="menu-node">
      <div className="menu-node-row">
        <div className="menu-node-main">
          {hasChildren ? (
            <button
              className="menu-expand"
              type="button"
              aria-label={expanded ? '收起菜单' : '展开菜单'}
              onClick={() => setExpanded((open) => !open)}
            >
              <ChevronRight className={expanded ? 'open' : ''} size={15} />
            </button>
          ) : (
            <span className="menu-expand-placeholder" />
          )}
          <div>
            <strong>{node.name}</strong>
            <span>{node.type === 'button' ? node.permission : node.path}</span>
          </div>
        </div>
        <RowActions
          canEdit={canEdit}
          canDelete={canDelete}
          onEdit={() => onEdit(node)}
          onDelete={() => onDelete(node.id)}
        />
      </div>
      {hasChildren && expanded && (
        <div className="menu-children">
          {node.children.map((child) => (
            <MenuNode
              key={child.id}
              node={child}
              onEdit={onEdit}
              onDelete={onDelete}
              canEdit={canEdit}
              canDelete={canDelete}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export function buildMenuTree(menus: Menu[]): MenuNodeType[] {
  const map = new Map<number, MenuNodeType>()
  menus.forEach((menu) => map.set(menu.id, { ...menu, children: [] }))
  const roots: MenuNodeType[] = []

  map.forEach((node) => {
    const parent = map.get(node.parentId)
    if (parent) {
      parent.children.push(node)
      return
    }
    roots.push(node)
  })

  return roots
}
