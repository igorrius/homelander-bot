import { Stack, Text, ThemeIcon } from '@mantine/core'
import type { Icon } from '@tabler/icons-react'
import type { ReactNode } from 'react'

/** What an empty list shows: an icon, what is (not) here, and optionally the next step. */
export function EmptyState({ icon: IconC, title, hint, action }: { icon: Icon; title: string; hint?: string; action?: ReactNode }) {
  return (
    <Stack align="center" gap="xs" py="xl" ta="center">
      <ThemeIcon variant="light" size={48} radius="xl" color="gray" aria-hidden>
        <IconC size={26} stroke={1.5} />
      </ThemeIcon>
      <Text fw={600}>{title}</Text>
      {hint && (
        <Text size="sm" c="dimmed" maw={420}>
          {hint}
        </Text>
      )}
      {action}
    </Stack>
  )
}
