import { BadgeCheck } from 'lucide-react'

export function FormActions({
  busy,
  editing,
  createPermission,
  editPermission,
  can,
  onReset,
}: {
  busy: boolean
  editing: boolean
  createPermission: string
  editPermission: string
  can: (permission: string) => boolean
  onReset: () => void
}) {
  const allowed = editing ? can(editPermission) : can(createPermission)
  return (
    <div className="form-actions">
      {allowed && (
        <button className="primary-button" disabled={busy} type="submit">
          <BadgeCheck size={15} />
          {editing ? '保存' : '新增'}
        </button>
      )}
      <button className="ghost-button" type="button" onClick={onReset}>
        重置
      </button>
    </div>
  )
}
