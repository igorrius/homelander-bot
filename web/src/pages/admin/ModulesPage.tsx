import { Alert, Button, Card, Group, Loader, Stack, Text, ThemeIcon, Title } from '@mantine/core'
import { IconArrowRight, IconNews, IconPuzzle, type Icon } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useAdminModules } from '../../api/modules'
import type { AdminModule } from '../../api/types'
import { EmptyState } from '../../components/EmptyState'
import { PageHeader } from '../../components/PageHeader'
import { errorText } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import { ModuleSwitch } from './ModuleSwitch'

/** Modules with an admin page of their own, and their icons. */
const modulePages: Record<string, { icon: Icon; to: string }> = {
  feed: { icon: IconNews, to: '/admin/modules/feed' },
}

function LastChange({ module: m }: { module: AdminModule }) {
  const { t, i18n } = useTranslation()
  const date = formatDateTime(m.updatedAt, i18n.language)
  const text = !date
    ? t(m.defaultEnabled ? 'admin.modules.defaultOn' : 'admin.modules.defaultOff')
    : m.updatedBy
      ? t('admin.modules.changed', { date, name: m.updatedBy.name })
      : t('admin.modules.changedAt', { date })
  return (
    <Text size="xs" c="dimmed">
      {text}
    </Text>
  )
}

function ModuleCard({ module: m }: { module: AdminModule }) {
  const { t } = useTranslation()
  const page = modulePages[m.code]
  const IconC = page?.icon ?? IconPuzzle
  return (
    <Card withBorder padding="lg" data-testid={`module-${m.code}`}>
      <Stack gap="md">
        <Group justify="space-between" align="center" gap="sm">
          <Group gap="sm" wrap="nowrap" align="center" style={{ minWidth: 0 }}>
            <ThemeIcon variant="light" size={40} radius="md" color={m.enabled ? 'teal' : 'gray'} aria-hidden>
              <IconC size={22} stroke={1.6} />
            </ThemeIcon>
            <Title order={3} size="h4">
              {t(m.nameKey)}
            </Title>
          </Group>
          <ModuleSwitch module={m} />
        </Group>
        <Text size="sm" c="dimmed" maw={620}>
          {t(m.descriptionKey)}
        </Text>
        <Group justify="space-between" align="center" gap="xs">
          <LastChange module={m} />
          {page && (
            <Button component={Link} to={page.to} variant="light" size="xs" rightSection={<IconArrowRight size={14} />}>
              {t('admin.modules.open')}
            </Button>
          )}
        </Group>
      </Stack>
    </Card>
  )
}

/** Built-in modules: one card each with its switch and a link to its page. */
export function ModulesPage() {
  const { t } = useTranslation()
  const modules = useAdminModules()
  return (
    <Stack>
      <PageHeader icon={IconPuzzle} title={t('admin.modules.title')} description={t('admin.modules.description')} />
      {modules.isPending ? (
        <Loader />
      ) : modules.isError ? (
        <Alert color="red">{errorText(t, modules.error)}</Alert>
      ) : modules.data.length === 0 ? (
        <EmptyState icon={IconPuzzle} title={t('admin.modules.empty')} />
      ) : (
        <Stack maw={760}>
          {modules.data.map((m) => (
            <ModuleCard key={m.code} module={m} />
          ))}
        </Stack>
      )}
    </Stack>
  )
}
