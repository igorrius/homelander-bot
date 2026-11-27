import { Alert, Anchor, Card, Group, List, Loader, SimpleGrid, Stack, Text, ThemeIcon, Title } from '@mantine/core'
import {
  IconBuildingCommunity,
  IconCircleDashed,
  IconHourglass,
  IconInbox,
  IconLayoutDashboard,
  IconUserCheck,
  type Icon,
} from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useHouses, useStatuses, useSummary, useUsers } from '../../api/admin'
import { useSite } from '../../api/public'
import { InviteLink } from '../../components/InviteLink'
import { PageHeader } from '../../components/PageHeader'
import { errorText } from '../../lib/errors'
import { ApplicationList } from './ApplicationList'

const latestCount = 5

function Stat({ to, icon: IconC, label, value, color }: { to: string; icon: Icon; label: string; value: number | undefined; color: string }) {
  return (
    <Card component={Link} to={to} padding="md" style={{ textDecoration: 'none', color: 'inherit' }}>
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Stack gap={4}>
          <Text size="sm" c="dimmed">
            {label}
          </Text>
          <Text fz={28} fw={700} lh={1}>
            {value ?? '—'}
          </Text>
        </Stack>
        <ThemeIcon variant="light" color={color} size={38} radius="md" aria-hidden>
          <IconC size={22} stroke={1.6} />
        </ThemeIcon>
      </Group>
    </Card>
  )
}

function SetupChecklist() {
  const { t } = useTranslation()
  const houses = useHouses()
  const statuses = useStatuses()
  const site = useSite()
  const noHouse = houses.isSuccess && houses.data.length === 0
  const noStatus = statuses.isSuccess && !statuses.data.some((s) => !s.archived)
  const noName = site.isSuccess && !Object.values(site.data.communityName).some((v) => v?.trim())
  // Shown only while registration cannot open; the name is suggested alongside, never on its own.
  if (!noHouse && !noStatus) return null
  const items: { to: string; label: string }[] = []
  if (noHouse) items.push({ to: '/admin/houses', label: t('admin.overview.setup.house') })
  if (noStatus) items.push({ to: '/admin/statuses', label: t('admin.overview.setup.status') })
  if (noName) items.push({ to: '/admin/site', label: t('admin.overview.setup.community') })
  return (
    <Alert color="yellow" variant="light" title={t('admin.overview.setup.title')}>
      <Stack gap="xs">
        <Text size="sm">{t('admin.overview.setup.hint')}</Text>
        <List spacing={4} size="sm" icon={<IconCircleDashed size={16} />} center>
          {items.map((it) => (
            <List.Item key={it.to}>
              <Anchor component={Link} to={it.to} size="sm">
                {it.label}
              </Anchor>
            </List.Item>
          ))}
        </List>
      </Stack>
    </Alert>
  )
}

function LatestApplications() {
  const { t } = useTranslation()
  const users = useUsers({ status: 'pending_approval', sort: 'newest', pageSize: latestCount })
  return (
    <Card>
      <Stack gap="sm">
        <Group justify="space-between">
          <Title order={3} size="h4">
            {t('admin.overview.latest')}
          </Title>
          <Anchor component={Link} to="/admin/applications" size="sm">
            {t('admin.overview.allApplications')}
          </Anchor>
        </Group>
        {users.isPending ? (
          <Loader size="sm" />
        ) : users.isError ? (
          <Alert color="red">{errorText(t, users.error)}</Alert>
        ) : users.data.items.length === 0 ? (
          <Text size="sm" c="dimmed">
            {t('admin.applications.empty')}
          </Text>
        ) : (
          <ApplicationList users={users.data.items} />
        )}
      </Stack>
    </Card>
  )
}

/** Admin home: setup reminders, counters, the registration link and the latest applications. */
export function OverviewPage() {
  const { t } = useTranslation()
  const summary = useSummary()
  const houses = useHouses()
  const site = useSite()
  const s = summary.data
  return (
    <Stack>
      <PageHeader icon={IconLayoutDashboard} title={t('admin.overview.title')} description={t('admin.overview.hint')} />
      <SetupChecklist />
      <SimpleGrid cols={{ base: 2, md: 4 }}>
        <Stat to="/admin/applications" icon={IconInbox} color="teal" label={t('admin.overview.stats.pending')} value={s?.pendingApplications} />
        <Stat to="/admin/users?status=awaiting_activation" icon={IconHourglass} color="yellow" label={t('admin.overview.stats.awaiting')} value={s?.awaitingActivation} />
        <Stat to="/admin/users?status=active" icon={IconUserCheck} color="blue" label={t('admin.overview.stats.active')} value={s?.activeUsers} />
        <Stat to="/admin/houses" icon={IconBuildingCommunity} color="grape" label={t('admin.overview.stats.houses')} value={houses.data?.length} />
      </SimpleGrid>
      <SimpleGrid cols={{ base: 1, lg: 2 }}>
        <Card>
          <Stack gap="sm">
            <Title order={3} size="h4">
              {t('admin.invite.title')}
            </Title>
            {site.data ? <InviteLink url={site.data.registerUrl} /> : site.isError ? <Alert color="red">{errorText(t, site.error)}</Alert> : <Loader size="sm" />}
          </Stack>
        </Card>
        <LatestApplications />
      </SimpleGrid>
    </Stack>
  )
}
