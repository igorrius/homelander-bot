import { Badge } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { UserStatus } from '../../api/types'

const colors: Record<UserStatus, string> = {
  pending_approval: 'yellow',
  awaiting_activation: 'blue',
  active: 'green',
  rejected: 'gray',
  disabled: 'red',
}

export function StatusBadge({ status }: { status: UserStatus }) {
  const { t } = useTranslation()
  return (
    <Badge color={colors[status]} variant="light">
      {t(`userStatus.${status}`)}
    </Badge>
  )
}
