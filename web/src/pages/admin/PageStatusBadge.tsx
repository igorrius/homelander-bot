import { Badge } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { PageStatus } from '../../api/types'

const colors: Record<PageStatus, string> = { published: 'green', draft: 'blue', archived: 'gray' }

export function PageStatusBadge({ status }: { status: PageStatus }) {
  const { t } = useTranslation()
  return <Badge variant="light" color={colors[status]}>{t(`admin.pages.status.${status}`)}</Badge>
}

