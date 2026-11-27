import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import en from './locales/en.json'
import uk from './locales/uk.json'

export const supportedLocales = ['uk', 'en'] as const
export type Locale = (typeof supportedLocales)[number]
export const defaultLocale: Locale = 'uk'
const storageKey = 'locale'

function storedLocale(): Locale {
  const saved = typeof localStorage === 'undefined' ? null : localStorage.getItem(storageKey)
  return (supportedLocales as readonly string[]).includes(saved ?? '') ? (saved as Locale) : defaultLocale
}

export const i18n = i18next.createInstance()

/** Initialises translations synchronously from the bundled locale files. */
export function initI18n() {
  if (!i18n.isInitialized) {
    void i18n.use(initReactI18next).init({
      resources: { uk: { translation: uk }, en: { translation: en } },
      lng: storedLocale(),
      fallbackLng: defaultLocale,
      supportedLngs: [...supportedLocales],
      interpolation: { escapeValue: false },
      initAsync: false,
    })
    i18n.on('languageChanged', (lng) => {
      document.documentElement.lang = lng
    })
    document.documentElement.lang = i18n.language
  }
  return i18n
}

/** Switches the language and remembers the choice in this browser. */
export async function setLocale(locale: Locale) {
  localStorage.setItem(storageKey, locale)
  await i18n.changeLanguage(locale)
}

initI18n()
