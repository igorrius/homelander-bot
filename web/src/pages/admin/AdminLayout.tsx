import { AppShell, Avatar, Box, Badge, Burger, Center, Group, Image, Loader, Menu, NavLink, ScrollArea, Stack, Text, Title, UnstyledButton } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import {
  IconBuildingCommunity,
  IconChevronDown,
  IconHistory,
  IconHome,
  IconInbox,
  IconLayoutDashboard,
  IconLogout,
  IconTags,
  IconUsers,
  IconWorld,
  type Icon,
} from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link, Navigate, Outlet, useLocation } from 'react-router'
import { useSummary } from '../../api/admin'
import { useLogout, useMe } from '../../api/public'
import type { Me } from '../../api/types'
import { LanguageSwitcher } from '../../components/LanguageSwitcher'
import { fullName } from '../../lib/format'

interface NavItem {
  to: string
  icon: Icon
  label: string
  badge?: number
  /** Active only on this exact path, not on the paths below it. */
  exact?: boolean
}

interface NavGroup {
  label?: string
  items: NavItem[]
}

function Nav({ onNavigate }: { onNavigate(): void }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const summary = useSummary()
  const pending = summary.data?.pendingApplications ?? 0
  const groups: NavGroup[] = [
    { items: [{ to: '/admin', icon: IconLayoutDashboard, label: t('admin.nav.overview'), exact: true }] },
    {
      label: t('admin.navGroup.people'),
      items: [
        { to: '/admin/applications', icon: IconInbox, label: t('admin.nav.applications'), badge: pending },
        { to: '/admin/users', icon: IconUsers, label: t('admin.nav.users') },
      ],
    },
    // M2 adds "Chats" to this group.
    { label: t('admin.navGroup.community'), items: [{ to: '/admin/houses', icon: IconBuildingCommunity, label: t('admin.nav.houses') }] },
    {
      label: t('admin.navGroup.settings'),
      items: [
        { to: '/admin/site', icon: IconWorld, label: t('admin.nav.site') },
        { to: '/admin/statuses', icon: IconTags, label: t('admin.nav.statuses') },
        { to: '/admin/audit', icon: IconHistory, label: t('admin.nav.audit') },
      ],
    },
  ]
  return (
    <Stack gap="lg">
      {groups.map((g, i) => (
        <Stack key={g.label ?? i} gap={2}>
          {g.label && (
            <Text size="xs" fw={600} c="dimmed" tt="uppercase" px="sm" pb={4}>
              {g.label}
            </Text>
          )}
          {g.items.map((it) => (
            <NavLink
              key={it.to}
              component={Link}
              to={it.to}
              label={it.label}
              leftSection={<it.icon size={18} stroke={1.6} />}
              active={it.exact ? pathname === it.to : pathname.startsWith(it.to)}
              onClick={onNavigate}
              rightSection={it.badge ? <Badge size="sm" circle>{it.badge}</Badge> : undefined}
            />
          ))}
        </Stack>
      ))}
    </Stack>
  )
}

function initials(me: Me): string {
  const s = `${me.firstName.charAt(0)}${me.lastName.charAt(0)}`.trim()
  return s || '·'
}

function UserMenu({ me }: { me: Me }) {
  const { t } = useTranslation()
  const logout = useLogout()
  return (
    <Menu position="bottom-end" width={220}>
      <Menu.Target>
        <UnstyledButton aria-label={fullName(me)}>
          <Group gap={6} wrap="nowrap">
            <Avatar size={30} radius="xl" color="teal">
              {initials(me)}
            </Avatar>
            <Text size="sm" fw={500} visibleFrom="sm">
              {fullName(me)}
            </Text>
            <Box visibleFrom="sm" lh={0}>
              <IconChevronDown size={14} />
            </Box>
          </Group>
        </UnstyledButton>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Item component={Link} to="/" leftSection={<IconHome size={16} />}>
          {t('admin.residentPage')}
        </Menu.Item>
        <Menu.Divider />
        <Menu.Item color="red" leftSection={<IconLogout size={16} />} disabled={logout.isPending} onClick={() => logout.mutate()}>
          {t('home.logout')}
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  )
}

/** Admin area frame: header, grouped menu with the applications counter, and the access guard. */
export function AdminLayout() {
  const { t } = useTranslation()
  const me = useMe()
  const [opened, { toggle, close }] = useDisclosure(false)
  if (me.isPending) {
    return (
      <Center h="100vh">
        <Loader />
      </Center>
    )
  }
  if (!me.data) return <Navigate to="/login" replace />
  if (!me.data.isAdmin) return <Navigate to="/" replace />
  return (
    <AppShell header={{ height: 60 }} navbar={{ width: 250, breakpoint: 'sm', collapsed: { mobile: !opened } }} padding="lg">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" aria-label={t('admin.menu')} />
            <Image src="/favicon.svg" alt="" w={28} h={28} />
            <Title order={1} size="h4" visibleFrom="xs">
              {t('admin.title')}
            </Title>
          </Group>
          <Group gap="sm" wrap="nowrap">
            <LanguageSwitcher />
            <UserMenu me={me.data} />
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="sm">
        <AppShell.Section grow component={ScrollArea}>
          <Nav onNavigate={close} />
        </AppShell.Section>
      </AppShell.Navbar>
      <AppShell.Main bg="light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-8))">
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
