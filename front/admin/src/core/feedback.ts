export type ToastKind = 'success' | 'error' | 'info'
export type ConfirmVariant = 'primary' | 'danger'

export type ToastDetail = {
  message: string
  kind?: ToastKind
}

export type ConfirmDetail = {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: ConfirmVariant
  resolve: (confirmed: boolean) => void
}

export const toastEventName = 'admin:toast'
export const confirmEventName = 'admin:confirm'

export function showToast(message: string, kind: ToastKind = 'info') {
  window.dispatchEvent(new CustomEvent<ToastDetail>(toastEventName, { detail: { message, kind } }))
}

export function showSuccess(message: string) {
  showToast(message, 'success')
}

export function showError(message: string) {
  showToast(message, 'error')
}

export function confirmAction(options: Omit<ConfirmDetail, 'resolve'>): Promise<boolean> {
  return new Promise((resolve) => {
    window.dispatchEvent(new CustomEvent<ConfirmDetail>(confirmEventName, {
      detail: { ...options, resolve },
    }))
  })
}
