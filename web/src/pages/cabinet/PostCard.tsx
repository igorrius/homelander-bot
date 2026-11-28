import { Anchor, Avatar, Card, Group, Image, Stack, Text } from '@mantine/core'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { CabinetPost } from '../../api/types'
import { LinkifiedText } from '../../components/LinkifiedText'
import { formatDateTime } from '../../lib/format'

export type PostView = Pick<CabinetPost, 'body' | 'imageUrl' | 'publishedAt' | 'authorName'>

/**
 * One feed post as residents read it: author and date, the image, then the
 * text with its line breaks and links. With to, the date links to the post's page.
 */
export function PostCard({ post, to, testId, compact }: { post: PostView; to?: string; testId?: string; /** A shorter card for the home page. */ compact?: boolean }) {
  const { t, i18n } = useTranslation()
  const date = formatDateTime(post.publishedAt, i18n.language)
  const time: ReactNode = (
    <time dateTime={post.publishedAt}>{date}</time>
  )
  return (
    <Card component="article" data-testid={testId}>
      <Stack gap="sm">
        <Group gap="sm" wrap="nowrap">
          <Avatar size={36} radius="xl" color="teal" aria-hidden>
            {post.authorName.trim().charAt(0).toUpperCase() || '·'}
          </Avatar>
          <Stack gap={0} style={{ minWidth: 0 }}>
            {post.authorName && (
              <Text size="sm" fw={600} truncate>
                {post.authorName}
              </Text>
            )}
            <Text size="xs" c="dimmed">
              {to ? (
                <Anchor component={Link} to={to} c="dimmed" inherit aria-label={`${t('cabinet.feed.open')}: ${date}`}>
                  {time}
                </Anchor>
              ) : (
                time
              )}
            </Text>
          </Stack>
        </Group>
        {post.imageUrl && (
          <Image src={post.imageUrl} alt={t('cabinet.feed.image')} radius="md" w="100%" h="auto" mah={compact ? 240 : 520} fit="cover" loading="lazy" decoding="async" />
        )}
        {post.body && <LinkifiedText text={post.body} lineClamp={compact ? 6 : undefined} />}
      </Stack>
    </Card>
  )
}
