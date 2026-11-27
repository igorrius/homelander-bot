import { Anchor, AppShell, Badge, Burger, Button, Center, Group, Loader, NavLink, Text, Title } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import { Link, Navigate, Outlet, useLocation } from 'react-router'
import { useSummary } from '../../api/admin'
import { useLogout, useMe } from '../../api/public'
import { LanguageSwitcher } from '../../components/LanguageSwitcher'
import { fullName } from '../../lib/format'

const sections = ['applications', 'users', 'houses', 'statuses', 'site', 'audit'] as const

function Nav({ onNavigate }: { onNavigate(): void }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const summary = useSummary()
  const pending = summary.data?.pendingApplications ?? 0
  return (
    <>
      {sections.map((s) => (
        <NavLink
          key={s}
          component={Link}
          to={`/admin/${s}`}
          label={t(`admin.nav.${s}`)}
          active={pathname.startsWith(`/admin/${s}`)}
          onClick={onNavigate}
          rightSection={s === 'applications' && pending > 0 ? <Badge size="sm" circle>{pending}</Badge> : undefined}
        />
      ))}
    </>
  )
}

/** Admin area frame: header, menu with the applications counter, and the access guard. */
export function AdminLayout() {
  const { t } = useTranslation()
  const me = useMe()
  const logout = useLogout()
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
    <AppShell header={{ height: 56 }} navbar={{ width: 240, breakpoint: 'sm', collapsed: { mobile: !opened } }} padding="md">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" aria-label={t('admin.menu')} />
            <Title order={1} size="h4">
              {t('admin.title')}
            </Title>
          </Group>
          <Group gap="sm" wrap="nowrap">
            <Anchor component={Link} to="/" size="sm" visibleFrom="sm">
              {t('admin.residentPage')}
            </Anchor>
            <Text size="sm" visibleFrom="sm">
              {fullName(me.data)}
            </Text>
            <LanguageSwitcher />
            <Button variant="default" size="xs" loading={logout.isPending} onClick={() => logout.mutate()}>
              {t('home.logout')}
            </Button>
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="md">
        <Nav onNavigate={close} />
      </AppShell.Navbar>
      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
