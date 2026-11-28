import { Alert, Card, Group, Loader, Stack, Text, ThemeIcon, Title } from '@mantine/core'
import { IconPlugConnectedX } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useAdminModules } from '../../api/modules'
import type { AdminModule } from '../../api/types'
import { errorText } from '../../lib/errors'
import { ModuleSwitch } from './ModuleSwitch'

/** What a module's admin page shows while the module is off: why, and the switch to turn it on. */
export function ModuleOff({ module: m }: { module: AdminModule }) {
  const { t } = useTranslation()
  return (
    <Card withBorder padding="lg" maw={640}>
      <Stack gap="md">
        <Group gap="sm" wrap="nowrap" align="flex-start">
          <ThemeIcon variant="light" color="gray" size={40} radius="md" aria-hidden>
            <IconPlugConnectedX size={22} stroke={1.6} />
          </ThemeIcon>
          <Stack gap={4}>
            <Title order={3} size="h4">
              {t('admin.moduleOff.title')}
            </Title>
            <Text size="sm" c="dimmed">
              {t('admin.moduleOff.text')}
            </Text>
          </Stack>
        </Group>
        <ModuleSwitch module={m} />
      </Stack>
    </Card>
  )
}

/** Renders children only while the module with this code is on; otherwise the turned-off card. */
export function ModuleGate({ code, children }: { code: string; children: ReactNode }) {
  const { t } = useTranslation()
  const modules = useAdminModules()
  if (modules.isPending) return <Loader />
  if (modules.isError) return <Alert color="red">{errorText(t, modules.error)}</Alert>
  const m = modules.data.find((x) => x.code === code)
  if (!m) return <Alert color="red">{t('errors.not_found')}</Alert>
  return m.enabled ? <>{children}</> : <ModuleOff module={m} />
}
