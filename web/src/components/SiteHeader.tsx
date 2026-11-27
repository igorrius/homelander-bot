import { Avatar, Box, Button, Container, Group, Image, Loader, Menu, Text, UnstyledButton, useMantineColorScheme, type MantineColorScheme } from '@mantine/core'
import { IconCheck, IconChevronDown, IconDeviceDesktop, IconHome2, IconLayoutDashboard, IconLogout, IconMoon, IconSun, type Icon } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useLogout, useMe } from '../api/public'
import type { Me } from '../api/types'
import { fullName } from '../lib/format'
import { LanguageSwitcher } from './LanguageSwitcher'

const themes: { value: MantineColorScheme; icon: Icon }[] = [
  { value: 'light', icon: IconSun },
  { value: 'dark', icon: IconMoon },
  { value: 'auto', icon: IconDeviceDesktop },
]

function initials(me: Me): string {
  return `${me.firstName.charAt(0)}${me.lastName.charAt(0)}`.trim() || '·'
}

function AccountMenu({ me }: { me: Me }) {
  const { t } = useTranslation()
  const logout = useLogout()
  const { colorScheme, setColorScheme } = useMantineColorScheme()
  return (
    <Menu position="bottom-end" width={240} shadow="md">
      <Menu.Target>
        <UnstyledButton aria-label={fullName(me)} style={{ borderRadius: 999 }}>
          <Group gap={6} wrap="nowrap">
            <Avatar size={32} radius="xl" color="teal" variant="filled">
              {initials(me)}
            </Avatar>
            <Text size="sm" fw={500} visibleFrom="xs" style={{ whiteSpace: 'nowrap' }}>
              {me.firstName}
            </Text>
            <Box lh={0} c="dimmed">
              <IconChevronDown size={14} />
            </Box>
          </Group>
        </UnstyledButton>
      </Menu.Target>
      <Menu.Dropdown>
        <Box px="sm" pt={6} pb={me.residencies.length ? 2 : 6}>
          <Text size="sm" fw={600}>
            {fullName(me)}
          </Text>
          {me.residencies.map((r) => (
            <Group key={r.id} gap={6} wrap="nowrap" mt={4}>
              <IconHome2 size={14} color="var(--mantine-color-dimmed)" />
              <Text size="xs" c="dimmed">
                {t('home.apartment', { house: r.houseName, n: r.apartment })}
              </Text>
            </Group>
          ))}
        </Box>
        {me.isAdmin && (
          <>
            <Menu.Divider />
            <Menu.Item component={Link} to="/admin" leftSection={<IconLayoutDashboard size={16} />}>
              {t('home.admin')}
            </Menu.Item>
          </>
        )}
        <Menu.Divider />
        <Menu.Label>{t('admin.theme.title')}</Menu.Label>
        {themes.map(({ value, icon: IconC }) => (
          <Menu.Item key={value} leftSection={<IconC size={16} />} rightSection={colorScheme === value ? <IconCheck size={14} /> : undefined}
            onClick={() => setColorScheme(value)}>
            {t(`admin.theme.${value}`)}
          </Menu.Item>
        ))}
        <Menu.Divider />
        <Menu.Item color="red" leftSection={<IconLogout size={16} />} disabled={logout.isPending} onClick={() => logout.mutate()}>
          {t('home.logout')}
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  )
}

function Account() {
  const { t } = useTranslation()
  const me = useMe()
  if (me.isPending) return <Loader size="sm" />
  if (!me.data) {
    return (
      <Group gap="sm" wrap="nowrap">
        <Text component={Link} to="/register" size="sm" fw={500} c="dimmed" visibleFrom="xs" style={{ whiteSpace: 'nowrap' }}>
          {t('home.register')}
        </Text>
        <Button component={Link} to="/login" size="xs" radius="xl">
          {t('home.login')}
        </Button>
      </Group>
    )
  }
  return <AccountMenu me={me.data} />
}

export const SITE_HEADER_HEIGHT = 60

/**
 * The one-line header over the built-in page and published pages: the community name (one line,
 * ellipsis), then the account control and the language switch. Nothing in it wraps.
 */
export function SiteHeader({ title }: { title: string }) {
  return (
    <Box
      component="header"
      style={{
        position: 'sticky',
        top: 0,
        zIndex: 100,
        height: SITE_HEADER_HEIGHT,
        borderBottom: '1px solid var(--mantine-color-default-border)',
        background: 'color-mix(in srgb, var(--mantine-color-body) 88%, transparent)',
        backdropFilter: 'saturate(1.6) blur(10px)',
      }}
    >
      <Container size="lg" h="100%">
        <Group h="100%" justify="space-between" wrap="nowrap" gap="md">
          <UnstyledButton component={Link} to="/" style={{ minWidth: 0, flex: '1 1 auto' }}>
            <Group gap={10} wrap="nowrap" style={{ minWidth: 0 }}>
              <Image src="/favicon.svg" alt="" w={28} h={28} style={{ flex: 'none' }} />
              <Text
                data-testid="site-header-name"
                title={title}
                fw={650}
                size="md"
                style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0 }}
              >
                {title}
              </Text>
            </Group>
          </UnstyledButton>
          <Group gap="sm" wrap="nowrap" style={{ flex: 'none' }}>
            <Account />
            <LanguageSwitcher />
          </Group>
        </Group>
      </Container>
    </Box>
  )
}
