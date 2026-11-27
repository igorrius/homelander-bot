import { useTranslation } from 'react-i18next'
import { PageShell } from '../components/PageShell'

export function LoginPage() {
  const { t } = useTranslation()
  return <PageShell title={t('login.title')} />
}
