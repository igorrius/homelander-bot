import { Alert, Skeleton, Stack } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { ApiError } from '../../api/client'
import { useCabinetPost } from '../../api/feed'
import { BackLink } from '../../components/BackLink'
import { errorText } from '../../lib/errors'
import { NotFoundPage } from '../NotFoundPage'
import { PostCard } from './PostCard'

/** One published post: where a Telegram copy's "Open in cabinet" button leads. */
export function FeedPostPage() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const n = Number(id)
  const valid = Number.isInteger(n) && n > 0
  const post = useCabinetPost(valid ? n : 0)
  if (!valid || (post.error instanceof ApiError && post.error.status === 404)) return <NotFoundPage />
  return (
    <Stack gap="md">
      <BackLink to="/my/feed">{t('cabinet.feed.back')}</BackLink>
      {post.isPending ? <Skeleton h={160} radius="md" /> : post.isError ? <Alert color="red">{errorText(t, post.error)}</Alert> : <PostCard post={post.data} />}
    </Stack>
  )
}
