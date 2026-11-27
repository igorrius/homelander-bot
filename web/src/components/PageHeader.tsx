import { Group, Stack, Text, ThemeIcon, Title } from '@mantine/core'
import type { Icon } from '@tabler/icons-react'
import type { ReactNode } from 'react'

/** The top of an admin page: icon, title, optional description and actions on the right. */
export function PageHeader({ icon: IconC, title, description, actions }: { icon?: Icon; title: string; description?: ReactNode; actions?: ReactNode }) {
  return (
    <Group justify="space-between" align="flex-start" gap="sm" mb="xs">
      <Group gap="sm" wrap="nowrap" align="center">
        {IconC && (
          <ThemeIcon variant="light" size={40} radius="md" aria-hidden>
            <IconC size={22} stroke={1.6} />
          </ThemeIcon>
        )}
        <Stack gap={2}>
          <Title order={2} size="h3">
            {title}
          </Title>
          {description && (
            <Text size="sm" c="dimmed">
              {description}
            </Text>
          )}
        </Stack>
      </Group>
      {actions && <Group gap="xs">{actions}</Group>}
    </Group>
  )
}
