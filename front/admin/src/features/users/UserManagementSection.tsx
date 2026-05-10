import type { FormEvent } from 'react'

import type { Role, User, UserForm } from '../../adminTypes'
import { CheckboxGroup, FormActions, PanelTitle, RowActions, formatNames } from '../../components/shared'

const emptyUser: UserForm = {
  username: '',
  nickname: '',
  password: '',
  roleIds: [],
}

export function UserManagementSection({
  users,
  roles,
  roleNameByID,
  userForm,
  saving,
  can,
  onUserFormChange,
  onSaveUser,
  onDeleteUser,
}: {
  users: User[]
  roles: Role[]
  roleNameByID: Map<number, string>
  userForm: UserForm
  saving: boolean
  can: (permission: string) => boolean
  onUserFormChange: (form: UserForm) => void
  onSaveUser: (event: FormEvent) => void
  onDeleteUser: (id: number) => void
}) {
  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title="用户列表" count={users.length} />
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>用户名</th>
                <th>昵称</th>
                <th>角色</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id}>
                  <td>{user.username}</td>
                  <td>{user.nickname || '-'}</td>
                  <td>{formatNames(user.roleIds, roleNameByID)}</td>
                  <td>
                    <RowActions
                      canEdit={can('user:edit')}
                      canDelete={can('user:delete')}
                      onEdit={() => onUserFormChange({ ...user, password: '' })}
                      onDelete={() => onDeleteUser(user.id)}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <form className="editor-panel" onSubmit={onSaveUser}>
        <PanelTitle title={userForm.id ? '编辑用户' : '新增用户'} />
        <label>
          用户名
          <input
            value={userForm.username}
            onChange={(event) =>
              onUserFormChange({ ...userForm, username: event.target.value })
            }
            placeholder="admin"
          />
        </label>
        <label>
          昵称
          <input
            value={userForm.nickname}
            onChange={(event) =>
              onUserFormChange({ ...userForm, nickname: event.target.value })
            }
            placeholder="管理员"
          />
        </label>
        <label>
          密码
          <input
            type="password"
            value={userForm.password}
            onChange={(event) =>
              onUserFormChange({ ...userForm, password: event.target.value })
            }
            placeholder={userForm.id ? '留空不修改' : '请输入密码'}
          />
        </label>
        <CheckboxGroup
          label="分配角色"
          items={roles}
          selected={userForm.roleIds}
          getLabel={(role) => role.name}
          onChange={(roleIds) => onUserFormChange({ ...userForm, roleIds })}
        />
        <FormActions
          busy={saving}
          editing={Boolean(userForm.id)}
          createPermission="user:create"
          editPermission="user:edit"
          can={can}
          onReset={() => onUserFormChange(emptyUser)}
        />
      </form>
    </section>
  )
}
