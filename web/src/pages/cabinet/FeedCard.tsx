import { Anchor, Group, Stack, Title } from '@mantine/core'
import { IconArrowRight, IconNews } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useCabinetFeed } from '../../api/feed'
import { useMyModules } from '../../api/modules'
import { PostCard } from './PostCard'

const shown = 3

/** The latest feed posts on top of the cabinet home; nothing while the feed is off or empty. */
export function FeedCard() {
  const { t } = useTranslation()
  const on = useMyModules().data?.some((m) => m.code === 'feed') ?? false
  const feed = useCabinetFeed(on)
  const latest = feed.data?.pages[0]?.items.slice(0, shown) ?? []
  if (!on || latest.length === 0) return null
  return (
    <Stack gap="sm" data-testid="cabinet-feed">
      <Group justify="space-between" align="center" wrap="nowrap">
        <Group gap="xs" wrap="nowrap">
          <IconNews size={20} stroke={1.7} color="var(--mantine-color-teal-filled)" aria-hidden />
          <Title order={2} size="h4">
            {t('cabinet.feed.latest')}
          </Title>
        </Group>
        <Anchor component={Link} to="/my/feed" size="sm" fw={500} style={{ display: 'inline-flex', alignItems: 'center', gap: 4, flex: 'none' }}>
          {t('cabinet.feed.allPosts')}
          <IconArrowRight size={15} stroke={1.8} aria-hidden />
        </Anchor>
      </Group>
      {latest.map((p) => (
        <PostCard key={p.id} post={p} to={`/my/feed/${p.id}`} compact />
      ))}
    </Stack>
  )
}
