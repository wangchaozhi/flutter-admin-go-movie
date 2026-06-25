import { useState } from 'react'
import { ChevronRight } from 'lucide-react'

import type { Menu } from '../../adminTypes'
import { RowActions } from '../../components/shared'
import { resolveIcon } from '../../iconRegistry'
import type { MenuNodeType } from './menuTreeModel'

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
  const Icon = resolveIcon(node.icon)

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
          <span className="menu-node-icon">
            <Icon size={14} />
          </span>
          <div>
            <strong>{node.name}</strong>
            <span>
              {node.type === 'button' ? node.permission : node.path}
              {' · '}
              排序 {node.sortOrder}
            </span>
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
