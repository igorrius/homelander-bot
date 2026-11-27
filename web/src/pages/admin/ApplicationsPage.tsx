import { Alert, Anchor, Button, Group, Loader, Stack, Table, Text } from '@mantine/core'
import { IconInbox } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useUserAction, useUsers } from '../../api/admin'
import type { AdminUser } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { errorText } from '../../lib/errors'
import { formatDateTime, fullName } from '../../lib/format'
import { PageHeader } from '../../components/PageHeader'

function Row({ u }: { u: AdminUser }) {
  const { t, i18n } = useTranslation()
  const action = useUserAction()
  const busy = action.isPending
  return (
    <Table.Tr>
      <Table.Td>
        <Anchor component={Link} to={`/admin/users/${u.id}`}>
          {fullName(u)}
        </Anchor>
      </Table.Td>
      <Table.Td>{u.phone}</Table.Td>
      <Table.Td>{u.residencies.map((r) => t('home.apartment', { house: r.houseName, n: r.apartment })).join('; ')}</Table.Td>
      <Table.Td>{formatDateTime(u.createdAt, i18n.language)}</Table.Td>
      <Table.Td>
        <Group gap="xs" wrap="nowrap">
          <Button size="xs" color="green" loading={busy} onClick={() => action.mutate({ id: u.id, action: 'approve' })}>
            {t('admin.applications.approve')}
          </Button>
          <ConfirmButton size="xs" color="red" variant="light" disabled={busy} message={t('admin.applications.rejectConfirm', { name: fullName(u) })}
            onConfirm={() => action.mutate({ id: u.id, action: 'reject' })}>
            {t('admin.applications.reject')}
          </ConfirmButton>
        </Group>
        {action.error && (
          <Text c="red" size="xs">
            {errorText(t, action.error)}
          </Text>
        )}
      </Table.Td>
    </Table.Tr>
  )
}

export function ApplicationsPage() {
  const { t } = useTranslation()
  const users = useUsers({ status: 'pending_approval', pageSize: 100 })
  return (
    <Stack>
      <PageHeader icon={IconInbox} title={t('admin.applications.title')} description={t('admin.applications.hint')} />
      {users.isPending ? (
        <Loader />
      ) : users.isError ? (
        <Alert color="red">{errorText(t, users.error)}</Alert>
      ) : users.data.items.length === 0 ? (
        <Text>{t('admin.applications.empty')}</Text>
      ) : (
        <Table.ScrollContainer minWidth={640}>
          <Table striped>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('admin.users.name')}</Table.Th>
                <Table.Th>{t('admin.users.phone')}</Table.Th>
                <Table.Th>{t('admin.users.apartments')}</Table.Th>
                <Table.Th>{t('admin.applications.appliedAt')}</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {users.data.items.map((u) => (
                <Row key={u.id} u={u} />
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
    </Stack>
  )
}
