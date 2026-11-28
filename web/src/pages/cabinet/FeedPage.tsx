import { Alert, Button, Card, Skeleton, Stack, Text, Title } from '@mantine/core'
import { IconNews } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { ApiError } from '../../api/client'
import { useCabinetFeed } from '../../api/feed'
import { EmptyState } from '../../components/EmptyState'
import { errorText } from '../../lib/errors'
import { NotFoundPage } from '../NotFoundPage'
import { PostCard } from './PostCard'

/** All published posts, newest first, 20 at a time. */
export function FeedPage() {
  const { t } = useTranslation()
  const feed = useCabinetFeed()
  if (feed.error instanceof ApiError && feed.error.status === 404) return <NotFoundPage />
  const items = feed.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <Stack gap="lg">
      <Stack gap={2}>
        <Title order={1} size="h2">
          {t('cabinet.feed.title')}
        </Title>
        <Text c="dimmed">{t('cabinet.feed.subtitle')}</Text>
      </Stack>
      {feed.isPending ? (
        <Stack>
          <Skeleton h={120} radius="md" />
          <Skeleton h={120} radius="md" />
        </Stack>
      ) : feed.isError && items.length === 0 ? (
        <Alert color="red">{errorText(t, feed.error)}</Alert>
      ) : items.length === 0 ? (
        <Card>
          <EmptyState icon={IconNews} title={t('cabinet.feed.empty')} />
        </Card>
      ) : (
        <Stack>
          {items.map((p) => (
            <PostCard key={p.id} post={p} to={`/my/feed/${p.id}`} testId={`feed-post-${p.id}`} />
          ))}
          {feed.isFetchNextPageError && <Alert color="red">{errorText(t, feed.error)}</Alert>}
          {feed.hasNextPage && (
            <Button variant="light" onClick={() => void feed.fetchNextPage()} loading={feed.isFetchingNextPage} style={{ alignSelf: 'center' }}>
              {t('cabinet.feed.showMore')}
            </Button>
          )}
        </Stack>
      )}
    </Stack>
  )
}
