import { useEffect } from 'react'
import { useSite } from '../api/public'
import { applyDefaultLocale } from '../i18n'

/** Switches to the site's default language for visitors who never chose one. */
export function SiteLocale() {
  const lang = useSite().data?.defaultLocale
  useEffect(() => {
    if (lang) applyDefaultLocale(lang)
  }, [lang])
  return null
}
