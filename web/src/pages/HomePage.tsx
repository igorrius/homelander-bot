import { useTranslation } from 'react-i18next'
import { PageShell } from '../components/PageShell'

export function HomePage() {
  const { t } = useTranslation()
  return <PageShell title={t('home.title')} />
}
