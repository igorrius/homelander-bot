import { useTranslation } from 'react-i18next'
import { PageShell } from '../components/PageShell'

export function RegisterPage() {
  const { t } = useTranslation()
  return <PageShell title={t('register.title')} />
}
