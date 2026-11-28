import { Badge, Group, Text } from '@mantine/core'
import type { Label } from '../../api/types'

/** The names of the labels with these ids, or `empty` when there are none. */
export function LabelBadges({ ids, labels, empty }: { ids: number[]; labels: Label[]; empty: string }) {
  if (ids.length === 0) return <Text size="xs" c="dimmed">{empty}</Text>
  return (
    <Group gap={4}>
      {ids.map((id) => (
        <Badge key={id} variant="light" radius="sm" tt="none">
          {labels.find((l) => l.id === id)?.name ?? `#${id}`}
        </Badge>
      ))}
    </Group>
  )
}
