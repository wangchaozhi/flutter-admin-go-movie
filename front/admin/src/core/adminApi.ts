import type { ApiResponse } from '../adminTypes'

export class AdminApiError extends Error {
  status: number
  code?: number

  constructor(message: string, status: number, code?: number) {
    super(message)
    this.name = 'AdminApiError'
    this.status = status
    this.code = code
  }
}

type AdminRequestOptions = RequestInit & {
  token?: string
}

export async function adminRequest<T>(url: string, options: AdminRequestOptions = {}): Promise<T> {
  const { token, headers: initHeaders, ...init } = options
  const headers = new Headers(initHeaders)
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const res = await fetch(url, { ...init, headers })
  const contentType = res.headers.get('Content-Type') ?? ''
  const payload = contentType.includes('application/json')
    ? ((await res.json()) as ApiResponse<T>)
    : null

  if (res.status === 401 && !url.includes('/api/admin/login')) {
    window.dispatchEvent(new CustomEvent('admin:unauthorized'))
  }

  if (!res.ok || (payload && payload.code !== 0)) {
    throw new AdminApiError(payload?.msg || res.statusText || '请求失败', res.status, payload?.code)
  }

  return payload?.data as T
}

export function authHeaders(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}
