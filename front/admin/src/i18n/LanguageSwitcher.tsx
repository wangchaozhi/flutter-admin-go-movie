import { Languages } from 'lucide-react'

import { LOCALES, useI18n } from './core'
import type { Locale } from './core'

export function LanguageSwitcher({ className = '' }: { className?: string }) {
  const { locale, setLocale, t } = useI18n()
  return (
    <label className={`ghost-button lang-switcher ${className}`.trim()} title={t('language.label')}>
      <Languages size={15} />
      <select
        aria-label={t('language.label')}
        value={locale}
        onChange={(event) => setLocale(event.target.value as Locale)}
      >
        {LOCALES.map((item) => (
          <option key={item.code} value={item.code}>
            {item.label}
          </option>
        ))}
      </select>
    </label>
  )
}
