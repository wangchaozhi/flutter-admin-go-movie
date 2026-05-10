import { Trash2, UserCog } from 'lucide-react'

export function RowActions({
  canEdit,
  canDelete,
  onEdit,
  onDelete,
}: {
  canEdit: boolean
  canDelete: boolean
  onEdit: () => void
  onDelete: () => void
}) {
  return (
    <div className="row-actions">
      {canEdit && (
        <button type="button" onClick={onEdit}>
          <UserCog size={14} />
          编辑
        </button>
      )}
      {canDelete && (
        <button className="danger" type="button" onClick={onDelete}>
          <Trash2 size={14} />
          删除
        </button>
      )}
      {!canEdit && !canDelete && <span className="muted-action">无权限</span>}
    </div>
  )
}
