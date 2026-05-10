import { Trash2 } from 'lucide-react'

import type { ConfirmDialogState } from '../../adminTypes'

export function ConfirmDialog({
  state,
  busy,
  onCancel,
}: {
  state: ConfirmDialogState | null
  busy: boolean
  onCancel: () => void
}) {
  if (!state) return null

  return (
    <div className="confirm-backdrop" role="presentation" onMouseDown={onCancel}>
      <section
        aria-modal="true"
        className="confirm-dialog"
        role="dialog"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div>
          <h2>{state.title}</h2>
          <p>{state.message}</p>
        </div>
        <div className="confirm-actions">
          <button className="ghost-button" disabled={busy} type="button" onClick={onCancel}>
            取消
          </button>
          <button className="primary-button danger-confirm" disabled={busy} type="button" onClick={state.onConfirm}>
            <Trash2 size={15} />
            {state.confirmLabel}
          </button>
        </div>
      </section>
    </div>
  )
}
