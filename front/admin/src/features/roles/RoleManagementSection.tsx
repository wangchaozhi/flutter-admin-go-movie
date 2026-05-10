import type { FormEvent } from 'react'

import type { Menu, Role, RoleForm } from '../../adminTypes'
import { CheckboxGroup, FormActions, PanelTitle, RowActions, formatNames } from '../../components/shared'

const emptyRole: RoleForm = {
  name: '',
  key: '',
  menuIds: [],
}

export function RoleManagementSection({
  roles,
  pageMenus,
  buttonMenus,
  menuNameByID,
  roleForm,
  saving,
  can,
  onRoleFormChange,
  onSaveRole,
  onDeleteRole,
}: {
  roles: Role[]
  pageMenus: Menu[]
  buttonMenus: Menu[]
  menuNameByID: Map<number, string>
  roleForm: RoleForm
  saving: boolean
  can: (permission: string) => boolean
  onRoleFormChange: (form: RoleForm) => void
  onSaveRole: (event: FormEvent) => void
  onDeleteRole: (id: number) => void
}) {
  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="角色列表" count={roles.length} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>角色</th>
                <th>标识</th>
                <th>菜单权限</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {roles.map((role) => (
                <tr key={role.id}>
                  <td>{role.name}</td>
                  <td>{role.key}</td>
                  <td>{formatNames(role.menuIds, menuNameByID)}</td>
                  <td>
                    <RowActions
                      canEdit={can('role:edit')}
                      canDelete={can('role:delete')}
                      onEdit={() => onRoleFormChange(role)}
                      onDelete={() => onDeleteRole(role.id)}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <form className="editor-panel" onSubmit={onSaveRole}>
        <PanelTitle title={roleForm.id ? '编辑角色' : '新增角色'} />
        <label>
          角色名称
          <input
            value={roleForm.name}
            onChange={(event) => onRoleFormChange({ ...roleForm, name: event.target.value })}
            placeholder="运营管理员"
          />
        </label>
        <label>
          角色标识
          <input
            value={roleForm.key}
            onChange={(event) => onRoleFormChange({ ...roleForm, key: event.target.value })}
            placeholder="operator"
          />
        </label>
        <CheckboxGroup
          label="菜单权限"
          items={pageMenus}
          selected={roleForm.menuIds}
          getLabel={(menu) => menu.name}
          onChange={(menuIds) => onRoleFormChange({ ...roleForm, menuIds })}
        />
        <CheckboxGroup
          label="按钮权限"
          items={buttonMenus}
          selected={roleForm.menuIds}
          getLabel={(menu) => `${menu.name} (${menu.permission})`}
          onChange={(menuIds) => onRoleFormChange({ ...roleForm, menuIds })}
        />
        <FormActions
          busy={saving}
          editing={Boolean(roleForm.id)}
          createPermission="role:create"
          editPermission="role:edit"
          can={can}
          onReset={() => onRoleFormChange(emptyRole)}
        />
      </form>
    </section>
  )
}
