import { Anchor, Button, Group, Paper, Stack, Text } from '@mantine/core'
import { IconCheck, IconX } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useUserAction } from '../../api/admin'
import type { AdminUser } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { errorText } from '../../lib/errors'
import { formatDateTime, fullName } from '../../lib/format'

function Item({ u }: { u: AdminUser }) {
  const { t, i18n } = useTranslation()
  const action = useUserAction()
  const busy = action.isPending
  return (
    <Paper withBorder p="sm">
      <Group justify="space-between" align="center" gap="sm" wrap="wrap">
        <Stack gap={2} style={{ minWidth: 0 }}>
          <Anchor component={Link} to={`/admin/users/${u.id}`} fw={600}>
            {fullName(u)}
          </Anchor>
          <Group gap="xs" c="dimmed" fz="sm" wrap="wrap">
            <Text span inherit>{u.phone}</Text>
            {u.residencies.length > 0 && (
              <Text span inherit>{u.residencies.map((r) => t('home.apartment', { house: r.houseName, n: r.apartment })).join('; ')}</Text>
            )}
            <Text span inherit>{formatDateTime(u.createdAt, i18n.language)}</Text>
          </Group>
        </Stack>
        <Group gap="xs" wrap="nowrap">
          <Button size="xs" color="green" leftSection={<IconCheck size={16} />} loading={busy} onClick={() => action.mutate({ id: u.id, action: 'approve' })}>
            {t('admin.applications.approve')}
          </Button>
          <ConfirmButton size="xs" color="red" variant="light" leftSection={<IconX size={16} />} disabled={busy}
            message={t('admin.applications.rejectConfirm', { name: fullName(u) })} onConfirm={() => action.mutate({ id: u.id, action: 'reject' })}>
            {t('admin.applications.reject')}
          </ConfirmButton>
        </Group>
      </Group>
      {action.error && (
        <Text c="red" size="xs" mt={4}>
          {errorText(t, action.error)}
        </Text>
      )}
    </Paper>
  )
}

/** Pending applications as rows that wrap on narrow screens, each with Approve and Reject. */
export function ApplicationList({ users }: { users: AdminUser[] }) {
  return (
    <Stack gap="xs">
      {users.map((u) => (
        <Item key={u.id} u={u} />
      ))}
    </Stack>
  )
}
