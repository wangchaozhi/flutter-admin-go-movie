import type { FormEvent } from 'react'

import type { Menu, MenuForm } from '../../adminTypes'
import { FormActions, PanelTitle } from '../../components/shared'
import { MenuNode } from './menuTree'
import type { MenuNodeType } from './menuTree'

const emptyMenu: MenuForm = {
  name: '',
  path: '',
  parentId: 0,
  type: 'menu',
  permission: '',
}

export function MenuManagementSection({
  menus,
  menuTree,
  pageMenus,
  menuForm,
  saving,
  can,
  onMenuFormChange,
  onSaveMenu,
  onDeleteMenu,
}: {
  menus: Menu[]
  menuTree: MenuNodeType[]
  pageMenus: Menu[]
  menuForm: MenuForm
  saving: boolean
  can: (permission: string) => boolean
  onMenuFormChange: (form: MenuForm) => void
  onSaveMenu: (event: FormEvent) => void
  onDeleteMenu: (id: number) => void
}) {
  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="菜单结构" count={menus.length} />
        <div className="menu-tree">
          {menuTree.map((node) => (
            <MenuNode
              key={node.id}
              node={node}
              onEdit={(menu) => onMenuFormChange(menu)}
              onDelete={onDeleteMenu}
              canEdit={can('menu:edit')}
              canDelete={can('menu:delete')}
            />
          ))}
        </div>
      </section>

      <form className="editor-panel" onSubmit={onSaveMenu}>
        <PanelTitle title={menuForm.id ? '编辑菜单' : '新增菜单'} />
        <label>
          类型
          <select
            value={menuForm.type}
            onChange={(event) =>
              onMenuFormChange({
                ...menuForm,
                type: event.target.value as MenuForm['type'],
                path: event.target.value === 'button' ? '' : menuForm.path,
                permission: event.target.value === 'menu' ? '' : menuForm.permission,
              })
            }
          >
            <option value="menu">菜单</option>
            <option value="button">按钮</option>
          </select>
        </label>
        <label>
          菜单名称
          <input
            value={menuForm.name}
            onChange={(event) => onMenuFormChange({ ...menuForm, name: event.target.value })}
            placeholder="系统管理"
          />
        </label>
        <label>
          路由路径
          <input
            disabled={menuForm.type === 'button'}
            value={menuForm.path}
            onChange={(event) => onMenuFormChange({ ...menuForm, path: event.target.value })}
            placeholder="/system/user"
          />
        </label>
        {menuForm.type === 'button' && (
          <label>
            权限标识
            <input
              value={menuForm.permission}
              onChange={(event) =>
                onMenuFormChange({ ...menuForm, permission: event.target.value })
              }
              placeholder="user:create"
            />
          </label>
        )}
        <label>
          上级菜单
          <select
            value={menuForm.parentId}
            onChange={(event) =>
              onMenuFormChange({ ...menuForm, parentId: Number(event.target.value) })
            }
          >
            <option value={0}>顶级菜单</option>
            {pageMenus
              .filter((menu) => menu.id !== menuForm.id)
              .map((menu) => (
                <option key={menu.id} value={menu.id}>
                  {menu.name}
                </option>
              ))}
          </select>
        </label>
        <FormActions
          busy={saving}
          editing={Boolean(menuForm.id)}
          createPermission="menu:create"
          editPermission="menu:edit"
          can={can}
          onReset={() => onMenuFormChange(emptyMenu)}
        />
      </form>
    </section>
  )
}
