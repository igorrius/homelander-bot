import { Anchor } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { PageShell } from '../components/PageShell'

export function NotFoundPage() {
  const { t } = useTranslation()
  return (
    <PageShell title={t('notFound.title')}>
      <Anchor component={Link} to="/">
        {t('notFound.home')}
      </Anchor>
    </PageShell>
  )
}
