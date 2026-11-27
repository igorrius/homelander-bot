import { useTranslation } from 'react-i18next'
import { useSite } from '../api/public'
import type { LocalizedText } from '../api/types'
import { defaultLocale } from '../i18n'

/**
 * Text for lang, else for the site's default locale, else any non-empty
 * value, else ''.
 */
export function localized(text: LocalizedText | undefined, lang: string, fallbackLocale: string): string {
  if (!text) return ''
  const values = text as Record<string, string | undefined>
  for (const loc of [lang, fallbackLocale]) {
    const v = values[loc]?.trim()
    if (v) return v
  }
  for (const v of Object.values(values)) {
    if (v?.trim()) return v.trim()
  }
  return ''
}

/** localized() bound to the current language and the site's default locale. */
export function useLocalized(): (text: LocalizedText | undefined) => string {
  const { i18n } = useTranslation()
  const site = useSite()
  const fallback = site.data?.defaultLocale ?? defaultLocale
  return (text) => localized(text, i18n.language, fallback)
}
