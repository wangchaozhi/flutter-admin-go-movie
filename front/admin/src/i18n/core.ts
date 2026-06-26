import { createContext, useContext } from 'react'

import { DEFAULT_LOCALE, LOCALES, messages } from './messages'
import type { Locale } from './messages'

export type { Locale } from './messages'
export { LOCALES, DEFAULT_LOCALE } from './messages'

export const LOCALE_STORAGE_KEY = 'admin.locale'

export function getStoredLocale(): Locale {
  const value = localStorage.getItem(LOCALE_STORAGE_KEY)
  if (LOCALES.some((item) => item.code === value)) return value as Locale
  // First visit: try to honor the browser language, else fall back to default.
  const nav = navigator.language?.toLowerCase() ?? ''
  if (nav.startsWith('en')) return 'en'
  if (nav.startsWith('ja')) return 'ja'
  if (nav.startsWith('zh') && (nav.includes('tw') || nav.includes('hk') || nav.includes('hant'))) return 'zh-TW'
  if (nav.startsWith('zh')) return 'zh'
  return DEFAULT_LOCALE
}

// resolve walks a dot-path ("invite.usesValue") into the nested dictionary.
function resolve(dict: Record<string, unknown>, path: string): string | undefined {
  let cur: unknown = dict
  for (const part of path.split('.')) {
    if (cur && typeof cur === 'object' && part in (cur as Record<string, unknown>)) {
      cur = (cur as Record<string, unknown>)[part]
    } else {
      return undefined
    }
  }
  return typeof cur === 'string' ? cur : undefined
}

// translate looks up `key` in the active locale, falling back to the default
// locale and finally the key itself, then interpolates {name} placeholders.
export function translate(locale: Locale, key: string, vars?: Record<string, string | number>): string {
  const found = resolve(messages[locale], key) ?? resolve(messages[DEFAULT_LOCALE], key) ?? key
  if (!vars) return found
  return found.replace(/\{(\w+)\}/g, (_, name: string) => String(vars[name] ?? `{${name}}`))
}

export type I18nContextValue = {
  locale: Locale
  setLocale: (locale: Locale) => void
  t: (key: string, vars?: Record<string, string | number>) => string
}

export const I18nContext = createContext<I18nContextValue | null>(null)

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used within I18nProvider')
  return ctx
}
