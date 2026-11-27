import { SegmentedControl } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { setLocale, supportedLocales, type Locale } from '../i18n'

const labels: Record<Locale, string> = { uk: 'УК', en: 'EN' }

export function LanguageSwitcher() {
  const { t, i18n } = useTranslation()
  return (
    <SegmentedControl
      aria-label={t('language.label')}
      size="xs"
      value={i18n.language}
      onChange={(v) => void setLocale(v as Locale)}
      data={supportedLocales.map((l) => ({ value: l, label: labels[l] }))}
    />
  )
}
