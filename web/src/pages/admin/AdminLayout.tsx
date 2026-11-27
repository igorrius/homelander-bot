import { AppShell, Group, Title } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { LanguageSwitcher } from '../../components/LanguageSwitcher'

/** Admin area frame; navigation and screens arrive with M1. */
export function AdminLayout() {
  const { t } = useTranslation()
  return (
    <AppShell header={{ height: 56 }} navbar={{ width: 240, breakpoint: 'sm' }} padding="md">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Title order={1} size="h3">
            {t('admin.title')}
          </Title>
          <LanguageSwitcher />
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="md" />
      <AppShell.Main />
    </AppShell>
  )
}
