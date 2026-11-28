import { AppShell, Box, Center, Container, Group, Loader, NavLink, Stack, Text, UnstyledButton } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconWorld, type Icon } from '@tabler/icons-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Navigate, Outlet, useLocation } from 'react-router'
import { useMe, useSite } from '../../api/public'
import { isSectionActive, useCabinetSections } from '../../cabinet/sections'
import { LanguageSwitcher } from '../../components/LanguageSwitcher'
import { Account, Brand } from '../../components/SiteHeader'
import { useLocalized } from '../../lib/localized'

/** Mantine's "sm" breakpoint: a sidebar from here up, a bottom bar below. */
const WIDE = '(min-width: 48em)'
const BOTTOM_BAR_HEIGHT = 64

interface Entry {
  key: string
  to: string
  icon: Icon
  label: string
  active: boolean
}

function useEntries(): Entry[] {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const sections = useCabinetSections()
  return [
    ...sections.map((s) => ({ key: s.key, to: s.to, icon: s.icon, label: t(s.labelKey), active: isSectionActive(s, pathname) })),
    { key: 'site', to: '/', icon: IconWorld, label: t('cabinet.nav.site'), active: false },
  ]
}

function SideNav({ entries }: { entries: Entry[] }) {
  return (
    <Stack gap={2}>
      {entries.map((e) => (
        <NavLink
          key={e.key}
          component={Link}
          to={e.to}
          label={e.label}
          leftSection={<e.icon size={18} stroke={1.6} />}
          active={e.active}
          aria-current={e.active ? 'page' : undefined}
        />
      ))}
    </Stack>
  )
}

function BottomNav({ entries, label }: { entries: Entry[]; label: string }) {
  return (
    <Box component="nav" aria-label={label} h="100%">
      <Group h="100%" gap={0} wrap="nowrap" grow>
        {entries.map((e) => (
          <UnstyledButton
            key={e.key}
            component={Link}
            to={e.to}
            aria-current={e.active ? 'page' : undefined}
            h="100%"
            c={e.active ? 'teal' : 'dimmed'}
            style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 2, minWidth: 0 }}
          >
            <e.icon size={24} stroke={e.active ? 2 : 1.6} />
            <Text size="xs" fw={e.active ? 650 : 500} truncate maw="100%">
              {e.label}
            </Text>
          </UnstyledButton>
        ))}
      </Group>
    </Box>
  )
}

/**
 * The resident cabinet frame and its guard: any signed-in (so active) user
 * gets in, a guest is sent to log in and brought back. Phones get a bottom
 * bar, wide screens a sidebar; both list the sections from useCabinetSections.
 */
export function CabinetLayout() {
  const { t } = useTranslation()
  const me = useMe()
  const site = useSite()
  const text = useLocalized()
  const { pathname, search } = useLocation()
  const wide = useMediaQuery(WIDE, false, { getInitialValueInEffect: false })
  const entries = useEntries()
  const communityName = text(site.data?.communityName)
  // iOS names a home-screen icon after this tag when the cabinet is added.
  useEffect(() => {
    if (communityName) document.querySelector('meta[name="apple-mobile-web-app-title"]')?.setAttribute('content', communityName)
  }, [communityName])
  if (me.isPending) {
    return (
      <Center h="100vh">
        <Loader />
      </Center>
    )
  }
  if (!me.data) return <Navigate to={'/login?next=' + encodeURIComponent(pathname + search)} replace />
  const name = communityName || t('cabinet.title')
  const navLabel = t('cabinet.nav.label')
  return (
    <AppShell
      header={{ height: 60 }}
      navbar={wide ? { width: 240, breakpoint: 'sm' } : undefined}
      footer={wide ? undefined : { height: BOTTOM_BAR_HEIGHT }}
      padding="md"
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap" gap="md">
          <Brand title={name} to="/my" testId="cabinet-header-name" />
          <Group gap="sm" wrap="nowrap" style={{ flex: 'none' }}>
            <Account />
            <LanguageSwitcher />
          </Group>
        </Group>
      </AppShell.Header>
      {wide ? (
        <AppShell.Navbar p="sm" aria-label={navLabel}>
          <SideNav entries={entries} />
        </AppShell.Navbar>
      ) : (
        <AppShell.Footer>
          <BottomNav entries={entries} label={navLabel} />
        </AppShell.Footer>
      )}
      <AppShell.Main bg="light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-8))">
        <Container size={640} px={0} py={{ base: 'xs', sm: 'md' }}>
          <Outlet />
        </Container>
      </AppShell.Main>
    </AppShell>
  )
}
