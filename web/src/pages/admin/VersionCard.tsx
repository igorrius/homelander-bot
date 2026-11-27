import { Anchor, Badge, Group, Paper, Stack, Text } from '@mantine/core'
import { IconRobot, IconUser } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { PageVersion } from '../../api/types'
import { formatDateTime, formatSize } from '../../lib/format'

/** The page addresses of a version as chips; on the live version they open the real site. */
export function PageChips({ version, linked }: { version: PageVersion; linked?: boolean }) {
  return (
    <Group gap={6}>
      {version.pages.map((p) =>
        linked ? (
          <Anchor key={p.path} href={p.path} target="_blank" rel="noopener" underline="never">
            <Badge variant="outline" color="green" radius="sm" tt="none" ff="monospace" style={{ cursor: 'pointer' }}>{p.path}</Badge>
          </Anchor>
        ) : (
          <Badge key={p.path} variant="default" radius="sm" tt="none" ff="monospace">{p.path}</Badge>
        ),
      )}
    </Group>
  )
}

/** Who made a version and how: an MCP badge (agent's client name as the tooltip) or a person. */
export function Author({ version }: { version: PageVersion }) {
  const { t, i18n } = useTranslation()
  return (
    <Group gap={6} wrap="wrap">
      {version.createdVia ? (
        <Badge size="sm" variant="light" color="grape" leftSection={<IconRobot size={12} />} title={version.createdVia}>
          {t('admin.pages.viaMcp')}
        </Badge>
      ) : (
        <IconUser size={14} color="var(--mantine-color-dimmed)" />
      )}
      <Text size="xs" c="dimmed">
        {t('admin.site.createdBy', { at: formatDateTime(version.createdAt, i18n.language), name: version.createdBy?.name ?? '—' })}
      </Text>
    </Group>
  )
}

/** One version as a card: note, author, pages, and the actions the caller allows. */
export function VersionCard({ version, actions, linked }: { version: PageVersion; actions?: ReactNode; linked?: boolean }) {
  const { t } = useTranslation()
  return (
    <Paper withBorder p="md" radius="md">
      <Group justify="space-between" align="flex-start" gap="sm" wrap="wrap">
        <Stack gap={6} style={{ minWidth: 0, flex: '1 1 260px' }}>
          <Group gap="xs" wrap="nowrap">
            <Text fw={600} truncate>{version.note || t('admin.site.version', { id: version.id })}</Text>
            {version.note && <Text size="xs" c="dimmed" style={{ flex: 'none' }}>#{version.id}</Text>}
          </Group>
          <Author version={version} />
          <Group gap="sm">
            <PageChips version={version} linked={linked} />
            <Text size="xs" c="dimmed">
              {t('admin.site.pageCount', { count: version.pages.length })} · {formatSize(version.totalSize)}
            </Text>
          </Group>
        </Stack>
        {actions && <Group gap="xs">{actions}</Group>}
      </Group>
    </Paper>
  )
}
