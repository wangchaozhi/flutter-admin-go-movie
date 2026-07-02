import { useEffect, useState } from 'react'

import type { ConfirmDialogState } from '../../adminTypes'
import { ConfirmDialog } from '../confirm'
import type { ConfirmDetail, ToastDetail, ToastKind } from '../../core/feedback'
import { confirmEventName, toastEventName } from '../../core/feedback'

type ToastItem = {
  id: number
  message: string
  kind: ToastKind
}

const toastTTL = 3600

export function FeedbackHost() {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const [confirm, setConfirm] = useState<ConfirmDetail | null>(null)

  useEffect(() => {
    function onToast(event: Event) {
      const detail = (event as CustomEvent<ToastDetail>).detail
      if (!detail?.message) return
      const id = Date.now() + Math.random()
      const item: ToastItem = { id, message: detail.message, kind: detail.kind ?? 'info' }
      setToasts((current) => [...current, item].slice(-4))
      window.setTimeout(() => {
        setToasts((current) => current.filter((toast) => toast.id !== id))
      }, toastTTL)
    }

    function onConfirm(event: Event) {
      const detail = (event as CustomEvent<ConfirmDetail>).detail
      if (!detail) return
      setConfirm(detail)
    }

    window.addEventListener(toastEventName, onToast)
    window.addEventListener(confirmEventName, onConfirm)
    return () => {
      window.removeEventListener(toastEventName, onToast)
      window.removeEventListener(confirmEventName, onConfirm)
    }
  }, [])

  function closeConfirm(confirmed: boolean) {
    confirm?.resolve(confirmed)
    setConfirm(null)
  }

  const confirmState: ConfirmDialogState | null = confirm
    ? {
        title: confirm.title,
        message: confirm.message,
        confirmLabel: confirm.confirmLabel ?? '确定',
        cancelLabel: confirm.cancelLabel,
        variant: confirm.variant,
        onConfirm: () => closeConfirm(true),
      }
    : null

  return (
    <>
      <ConfirmDialog state={confirmState} busy={false} onCancel={() => closeConfirm(false)} />
      <div className="toast-stack" aria-live="polite" aria-atomic="false">
        {toasts.map((toast) => (
          <button
            className={`toast toast-${toast.kind}`}
            key={toast.id}
            type="button"
            onClick={() => setToasts((current) => current.filter((item) => item.id !== toast.id))}
          >
            {toast.message}
          </button>
        ))}
      </div>
    </>
  )
}
