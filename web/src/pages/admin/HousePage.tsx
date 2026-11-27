import { Alert, Anchor, Button, Card, Drawer, List, Loader, SimpleGrid, Stack, Table, Text, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router'
import { useApartment, useHouse, useStatuses, useUpdateHouse } from '../../api/admin'
import { userStatuses, type House, type HouseStats } from '../../api/types'
import { errorText } from '../../lib/errors'
import { fullName } from '../../lib/format'
import { useLocalized } from '../../lib/localized'
import { HouseForm } from './HouseForm'
import { StatusBadge } from './StatusBadge'

function Stats({ stats }: { stats: HouseStats }) {
  const { t } = useTranslation()
  const text = useLocalized()
  const statuses = useStatuses()
  const byResidency = (statuses.data ?? [])
    .map((s) => ({ name: text(s.names) || s.code, n: stats.byResidencyStatus[String(s.id)] ?? 0 }))
    .filter((x) => x.n > 0)
  const byUser = userStatuses.map((s) => ({ s, n: stats.byUserStatus[s] ?? 0 })).filter((x) => x.n > 0)
  return (
    <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }}>
      <Card withBorder>
        <Text size="sm" c="dimmed">{t('admin.house.withResidents')}</Text>
        <Text size="xl" fw={700}>{`${stats.apartmentsWithResidents} / ${stats.apartmentsTotal}`}</Text>
      </Card>
      <Card withBorder>
        <Text size="sm" c="dimmed">{t('admin.house.people')}</Text>
        <Text size="xl" fw={700}>{stats.people}</Text>
      </Card>
      <Card withBorder>
        <Text size="sm" c="dimmed">{t('admin.house.byResidencyStatus')}</Text>
        <List size="sm" listStyleType="none">
          {byResidency.map((x) => <List.Item key={x.name}>{`${x.name}: ${x.n}`}</List.Item>)}
        </List>
      </Card>
      <Card withBorder>
        <Text size="sm" c="dimmed">{t('admin.house.byUserStatus')}</Text>
        <List size="sm" listStyleType="none">
          {byUser.map((x) => <List.Item key={x.s}>{`${t(`userStatus.${x.s}`)}: ${x.n}`}</List.Item>)}
        </List>
      </Card>
    </SimpleGrid>
  )
}

function Residents({ houseId, n, onClose }: { houseId: number; n: number | null; onClose(): void }) {
  const { t } = useTranslation()
  const text = useLocalized()
  const list = useApartment(houseId, n)
  return (
    <Drawer opened={n !== null} onClose={onClose} position="right" title={t('admin.house.apartmentTitle', { n })}>
      {list.isPending ? (
        <Loader />
      ) : list.isError ? (
        <Alert color="red">{errorText(t, list.error)}</Alert>
      ) : (
        <Table>
          <Table.Tbody>
            {list.data.map(({ user, residency }) => (
              <Table.Tr key={residency.id}>
                <Table.Td>
                  <Anchor component={Link} to={`/admin/users/${user.id}`}>{fullName(user)}</Anchor>
                  <Text size="xs" c="dimmed">{user.phone}</Text>
                </Table.Td>
                <Table.Td>{text(residency.statusNames)}</Table.Td>
                <Table.Td><StatusBadge status={user.status} /></Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Drawer>
  )
}

function Grid({ house, stats, onOpen }: { house: House; stats: HouseStats; onOpen(n: number): void }) {
  const { t } = useTranslation()
  const numbers = Array.from({ length: Math.max(0, house.apartmentTo - house.apartmentFrom + 1) }, (_, i) => house.apartmentFrom + i)
  const cells = numbers.map((n) => {
    const count = stats.apartments[String(n)] ?? 0
    return (
      <Button key={n} size="compact-sm" variant={count > 0 ? 'filled' : 'default'} disabled={count === 0}
        aria-label={t('admin.house.apartmentCell', { n, count })} onClick={() => onOpen(n)} styles={{ label: { flexDirection: 'column', lineHeight: 1.1 } }} h={44}>
        <span>{n}</span>
        {count > 0 && <small>{count}</small>}
      </Button>
    )
  })
  return (
    <div role="group" aria-label={t('admin.house.grid')} style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(52px, 1fr))', gap: 6 }}>
      {cells}
    </div>
  )
}

export function HousePage() {
  const { t } = useTranslation()
  const id = Number(useParams().id)
  const detail = useHouse(id)
  const update = useUpdateHouse(id)
  const [open, setOpen] = useState<number | null>(null)
  if (detail.isPending) return <Loader />
  if (detail.isError) return <Alert color="red">{errorText(t, detail.error)}</Alert>
  const { house, stats } = detail.data
  return (
    <Stack>
      <Anchor component={Link} to="/admin/houses" size="sm">
        ← {t('admin.nav.houses')}
      </Anchor>
      <Title order={2}>{house.name}</Title>
      <Stats stats={stats} />
      <Card withBorder>
        <Stack>
          <Title order={3}>{t('admin.house.grid')}</Title>
          <Grid house={house} stats={stats} onOpen={setOpen} />
        </Stack>
      </Card>
      <Card withBorder>
        <Stack>
          <Title order={3}>{t('admin.house.settings')}</Title>
          <HouseForm key={JSON.stringify(house)} initial={house} busy={update.isPending} error={update.error}
            onSave={(v) => update.mutate(v, { onSuccess: () => notifications.show({ message: t('common.saved'), color: 'green' }) })} />
        </Stack>
      </Card>
      <Residents houseId={id} n={open} onClose={() => setOpen(null)} />
    </Stack>
  )
}
