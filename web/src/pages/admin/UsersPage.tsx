import { Alert, Anchor, Badge, Button, Card, Group, Loader, Pagination, Select, Stack, Table, Tabs, TextInput, Tooltip } from '@mantine/core'
import { useDebouncedCallback, useDisclosure } from '@mantine/hooks'
import { useState } from 'react'
import { IconBrandTelegram, IconUserPlus, IconUserSearch, IconUsers } from '@tabler/icons-react'
import { EmptyState } from '../../components/EmptyState'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { useHouses, useSummary, useUsers, type UserQuery } from '../../api/admin'
import { userStatuses, type UserStatus } from '../../api/types'
import { errorText } from '../../lib/errors'
import { fullName } from '../../lib/format'
import { AddUserModal } from './AddUserModal'
import { StatusBadge } from './StatusBadge'
import { PageHeader } from '../../components/PageHeader'

const pageSize = 25
const boolParam = (v: string | null) => (v === 'true' ? true : v === 'false' ? false : undefined)
const numParam = (v: string | null) => (v && Number(v) > 0 ? Number(v) : undefined)

export function UsersPage() {
  const { t } = useTranslation()
  const [params, setParams] = useSearchParams()
  const [search, setSearch] = useState(params.get('q') ?? '')
  const [adding, { open, close }] = useDisclosure(false)
  const houses = useHouses(true)

  const q: UserQuery = {
    q: params.get('q') || undefined,
    houseId: numParam(params.get('houseId')),
    status: (userStatuses as string[]).includes(params.get('status') ?? '') ? (params.get('status') as UserStatus) : undefined,
    isAdmin: boolParam(params.get('isAdmin')),
    isResident: boolParam(params.get('isResident')),
    page: numParam(params.get('page')) ?? 1,
    pageSize,
  }
  const users = useUsers(q)
  const summary = useSummary()
  const sd = summary.data
  const counts: Partial<Record<UserStatus, number>> = sd
    ? { pending_approval: sd.pendingApplications, awaiting_activation: sd.awaitingActivation, active: sd.activeUsers, disabled: sd.disabledUsers, rejected: sd.rejectedUsers }
    : {}

  /** Sets one filter and goes back to page 1. */
  const setParam = (key: string, value: string | null) => {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value) next.set(key, value)
        else next.delete(key)
        if (key !== 'page') next.set('page', '1')
        return next
      },
      { replace: true },
    )
  }
  const searchLater = useDebouncedCallback((text: string) => {
    if (text.trim() !== (params.get('q') ?? '')) setParam('q', text.trim() || null)
  }, 300)

  const yesNo = [
    { value: 'true', label: t('common.yes') },
    { value: 'false', label: t('common.no') },
  ]
  const total = users.data?.total ?? 0
  return (
    <Stack>
      <PageHeader icon={IconUsers} title={t('admin.users.title')}
        actions={<Button leftSection={<IconUserPlus size={16} />} onClick={open}>{t('admin.users.add')}</Button>} />
      <Tabs variant="pills" value={q.status ?? 'all'} onChange={(v) => setParam('status', v && v !== 'all' ? v : null)}>
        <Tabs.List>
          <Tabs.Tab value="all">{t('admin.users.tabs.all')}</Tabs.Tab>
          {userStatuses.map((s) => (
            <Tabs.Tab key={s} value={s} rightSection={counts[s] !== undefined ? <Badge size="xs" variant="light" circle>{counts[s]}</Badge> : undefined}>
              {t(`userStatus.${s}`)}
            </Tabs.Tab>
          ))}
        </Tabs.List>
      </Tabs>
      <Group align="flex-end">
        <TextInput label={t('admin.users.search')} placeholder={t('admin.users.searchHint')} value={search}
          onChange={(e) => {
            setSearch(e.currentTarget.value)
            searchLater(e.currentTarget.value)
          }} w={260} />
        <Select label={t('admin.users.house')} placeholder={t('common.any')} clearable value={params.get('houseId')}
          data={(houses.data ?? []).map((h) => ({ value: String(h.id), label: h.name }))} onChange={(v) => setParam('houseId', v)} w={180} />
        <Select label={t('admin.users.isAdmin')} placeholder={t('common.any')} clearable value={params.get('isAdmin')}
          data={yesNo} onChange={(v) => setParam('isAdmin', v)} w={130} />
        <Select label={t('admin.users.isResident')} placeholder={t('common.any')} clearable value={params.get('isResident')}
          data={yesNo} onChange={(v) => setParam('isResident', v)} w={130} />
      </Group>
      {users.isPending ? (
        <Loader />
      ) : users.isError ? (
        <Alert color="red">{errorText(t, users.error)}</Alert>
      ) : users.data.items.length === 0 ? (
        <Card>
          <EmptyState icon={IconUserSearch} title={t('admin.users.empty')} />
        </Card>
      ) : (
        <Card p={0}>
        <Table.ScrollContainer minWidth={720}>
          <Table striped>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('admin.users.name')}</Table.Th>
                <Table.Th>{t('admin.users.phone')}</Table.Th>
                <Table.Th>{t('admin.users.status')}</Table.Th>
                <Table.Th>{t('admin.users.flags')}</Table.Th>
                <Table.Th>{t('admin.users.telegram')}</Table.Th>
                <Table.Th>{t('admin.users.apartments')}</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {users.data.items.map((u) => (
                <Table.Tr key={u.id}>
                  <Table.Td>
                    <Anchor component={Link} to={`/admin/users/${u.id}`}>
                      {fullName(u)}
                    </Anchor>
                  </Table.Td>
                  <Table.Td>{u.phone}</Table.Td>
                  <Table.Td>
                    <StatusBadge status={u.status} />
                  </Table.Td>
                  <Table.Td>
                    <Group gap={4}>
                      {u.isResident && <Badge variant="light">{t('admin.user.isResident')}</Badge>}
                      {u.isAdmin && <Badge variant="light" color="grape">{t('admin.user.isAdmin')}</Badge>}
                    </Group>
                  </Table.Td>
                  <Table.Td>
                    {u.hasTelegram && (
                      <Tooltip label={t('admin.users.linked')} withArrow>
                        <IconBrandTelegram size={18} stroke={1.6} role="img" aria-label={t('admin.users.linked')} color="var(--mantine-color-blue-6)" />
                      </Tooltip>
                    )}
                  </Table.Td>
                  <Table.Td>{u.residencies.map((r) => t('home.apartment', { house: r.houseName, n: r.apartment })).join('; ')}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
        </Card>
      )}
      {total > pageSize && (
        <Pagination total={Math.ceil(total / pageSize)} value={q.page} onChange={(p) => setParam('page', String(p))} />
      )}
      <AddUserModal opened={adding} onClose={close} />
    </Stack>
  )
}
