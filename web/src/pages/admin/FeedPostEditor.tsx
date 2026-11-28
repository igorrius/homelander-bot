import { Alert, Badge, Button, Card, FileButton, Grid, Group, Image, Input, Loader, MultiSelect, Stack, Text, Textarea, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconInfoCircle, IconNews, IconPhoto, IconSend, IconTrash } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { useChats, useLabels } from '../../api/chats'
import { adminAssetUrl, useUploadDataImage } from '../../api/data'
import { useCreatePost, useDeletePost, useFeedPost, useFeedSettings, usePublishPost, useUpdatePost, type PostInput } from '../../api/feed'
import { useMe } from '../../api/public'
import type { AdminPost, DeliveryState, Label } from '../../api/types'
import { BackLink } from '../../components/BackLink'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageHeader } from '../../components/PageHeader'
import { reachesChat } from '../../lib/chats'
import { errorText, useApiErrors } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import { PostCard } from '../cabinet/PostCard'
import { PostStateBadge } from './FeedAdminPage'
import { ModuleGate } from './ModuleOff'

/** Telegram allows 4096 characters in a message and 1024 in a photo caption; the backend keeps a margin. */
const maxBody = 4000
const maxCaption = 1000
const imageTypes = 'image/png,image/jpeg,image/webp,image/gif'
const deliveryColors: Record<DeliveryState, string> = { pending: 'yellow', sent: 'teal', failed: 'red' }

const postPath = (id: number) => `/admin/modules/feed/posts/${id}`
const sameIds = (a: number[], b: number[]) => a.length === b.length && a.every((x, i) => x === b[i])

/** Where each Telegram copy of a published post went and how it went. */
function Deliveries({ post }: { post: AdminPost }) {
  const { t, i18n } = useTranslation()
  const rows = post.deliveries ?? []
  let body
  if (post.labelIds.length === 0) body = <Text size="sm" c="dimmed">{t('admin.feed.deliveries.cabinetOnly')}</Text>
  else if (post.deliveryExpired) body = <Text size="sm" c="dimmed">{t('admin.feed.deliveries.expired')}</Text>
  else if (rows.length === 0) body = <Text size="sm" c="dimmed">{t('admin.feed.deliveries.waiting')}</Text>
  else {
    body = (
      <Stack gap="sm">
        {rows.map((d) => (
          <Stack key={d.id} gap={2} data-testid={`delivery-${d.id}`}>
            <Group justify="space-between" wrap="nowrap" gap="xs">
              <Text size="sm" fw={500} style={{ minWidth: 0 }} truncate>
                {d.topicTitle ? `${d.chatTitle} › ${d.topicTitle}` : d.chatTitle}
              </Text>
              <Badge variant="light" color={deliveryColors[d.state]} style={{ flex: 'none' }}>
                {t(`admin.feed.deliveries.state.${d.state}`)}
              </Badge>
            </Group>
            {d.error && <Text size="xs" c="red">{d.error}</Text>}
            {d.sentAt && <Text size="xs" c="dimmed">{formatDateTime(d.sentAt, i18n.language)}</Text>}
          </Stack>
        ))}
      </Stack>
    )
  }
  return (
    <Card withBorder>
      <Stack gap="sm">
        <Title order={3} size="h5">
          {t('admin.feed.deliveries.title')}
        </Title>
        {body}
      </Stack>
    </Card>
  )
}

interface EditorProps {
  /** Absent for a new post. */
  post?: AdminPost
  /** Label ids the form starts with. */
  initialLabels: number[]
  /** Labels the admin may choose (and those the post already has). */
  offered: Label[]
}

function Editor({ post, initialLabels, offered }: EditorProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const me = useMe()
  const create = useCreatePost()
  const update = useUpdatePost()
  const publish = usePublishPost()
  const del = useDeletePost()
  const upload = useUploadDataImage()
  const [body, setBody] = useState(post?.body ?? '')
  const [imageAssetId, setImageAssetId] = useState<number | null>(post?.imageAssetId ?? null)
  const [labelIds, setLabelIds] = useState<string[]>(initialLabels.map(String))
  const [now] = useState(() => new Date().toISOString())
  const published = post?.state === 'published'
  const ids = labelIds.map(Number)
  const length = [...body].length
  const max = imageAssetId !== null ? maxCaption : maxBody
  const tooLong = length > max
  const valid = body.trim() !== '' && !tooLong
  const dirty = !post || body !== post.body || imageAssetId !== post.imageAssetId || (!published && !sameIds(ids, post.labelIds))
  const busy = create.isPending || update.isPending || publish.isPending || del.isPending
  const lastError = create.error ?? update.error ?? publish.error ?? del.error ?? upload.error
  const { message, fields } = useApiErrors(lastError)
  const input = (): PostInput => (published ? { body: body.trim(), imageAssetId } : { body: body.trim(), imageAssetId, labelIds: ids })
  const names = ids.map((id) => offered.find((l) => l.id === id)?.name ?? `#${id}`)

  const save = () => {
    if (post) update.mutate({ id: post.id, input: input() })
    else create.mutate(input(), { onSuccess: (p) => navigate(postPath(p.id), { replace: true }) })
  }
  const doPublish = async () => {
    let id = post?.id
    try {
      if (!post) id = (await create.mutateAsync(input())).id
      else if (dirty) await update.mutateAsync({ id: post.id, input: input() })
      if (id === undefined) return
      await publish.mutateAsync({ id, labelIds: ids })
      if (!post) navigate(postPath(id), { replace: true })
    } catch (err) {
      // A new post is kept as a draft even when publishing it failed: open it.
      if (!post && id !== undefined) {
        notifications.show({ color: 'red', message: errorText(t, err) })
        navigate(postPath(id), { replace: true })
      }
    }
  }
  const remove = () => post && del.mutate(post.id, { onSuccess: () => navigate('/admin/modules/feed') })

  const author = post ? (post.createdBy?.name.split(' ')[0] ?? '') : (me.data?.firstName ?? '')
  const preview = { body: body.trim(), imageUrl: imageAssetId !== null ? adminAssetUrl(imageAssetId) : null, publishedAt: post?.publishedAt ?? now, authorName: author }
  const title = !post ? t('admin.feed.editor.titleNew') : published ? t('admin.feed.editor.titlePublished') : t('admin.feed.editor.titleDraft')
  const saved = (update.isSuccess && !dirty) || false

  return (
    <Stack>
      <PageHeader icon={IconNews} title={title} actions={post && <PostStateBadge post={post} />}
        description={post?.createdBy ? t('admin.feed.editor.author', { name: post.createdBy.name }) : undefined} />
      {published && (
        <Alert color="blue" icon={<IconInfoCircle size={18} />} maw={760}>
          {t('admin.feed.editor.publishedNote')}
        </Alert>
      )}
      <Grid gap="lg">
        <Grid.Col span={{ base: 12, md: 7 }}>
          <Card withBorder>
            <Stack>
              <Stack gap={4}>
                <Textarea label={t('admin.feed.editor.body')} description={t('admin.feed.editor.bodyHint')} value={body} required
                  onChange={(e) => setBody(e.currentTarget.value)} autosize minRows={6} maxRows={20}
                  error={tooLong ? t(imageAssetId !== null ? 'admin.feed.editor.tooLongWithImage' : 'admin.feed.editor.tooLong', { max }) : fields.body} />
                <Text size="xs" c={tooLong ? 'red' : 'dimmed'} ta="right" aria-live="polite">
                  {t('admin.feed.editor.counter', { count: length, max })}
                </Text>
              </Stack>
              <Input.Wrapper label={t('admin.feed.editor.image')} error={fields.imageAssetId ?? fields.file}>
                {imageAssetId !== null ? (
                  <Group mt={6} align="flex-end" gap="sm">
                    <Image src={adminAssetUrl(imageAssetId)} alt={t('cabinet.feed.image')} h={120} w="auto" maw="100%" fit="contain" radius="sm" />
                    <Group gap="xs">
                      <FileButton accept={imageTypes} onChange={(f) => f && upload.mutate(f, { onSuccess: (a) => setImageAssetId(a.id) })}>
                        {(props) => <Button {...props} variant="default" size="xs" loading={upload.isPending}>{t('admin.feed.editor.replaceImage')}</Button>}
                      </FileButton>
                      <Button variant="subtle" color="red" size="xs" onClick={() => setImageAssetId(null)}>
                        {t('admin.feed.editor.removeImage')}
                      </Button>
                    </Group>
                  </Group>
                ) : (
                  <Group mt={6}>
                    <FileButton accept={imageTypes} onChange={(f) => f && upload.mutate(f, { onSuccess: (a) => setImageAssetId(a.id) })}>
                      {(props) => (
                        <Button {...props} variant="default" leftSection={<IconPhoto size={16} />} loading={upload.isPending}>
                          {t('admin.feed.editor.uploadImage')}
                        </Button>
                      )}
                    </FileButton>
                  </Group>
                )}
              </Input.Wrapper>
              <MultiSelect label={t('admin.feed.editor.labels')}
                description={published ? t('admin.feed.editor.labelsFixed') : t('admin.feed.editor.labelsHint')}
                placeholder={labelIds.length === 0 ? t('admin.feed.editor.labelsPlaceholder') : undefined}
                data={offered.map((l) => ({ value: String(l.id), label: l.name }))} value={labelIds} onChange={setLabelIds}
                disabled={published} clearable={!published} error={fields.labelIds} name="labelIds" />
              {message && <Alert color="red">{message}</Alert>}
              {saved && <Text size="sm" c="teal">{t('admin.feed.editor.saved')}</Text>}
              <Group justify="space-between" gap="sm">
                <Group>
                  {post && (
                    <ConfirmButton color="red" variant="subtle" leftSection={<IconTrash size={16} />} disabled={busy}
                      message={published ? t('admin.feed.editor.deleteConfirmPublished') : t('admin.feed.editor.deleteConfirm')} onConfirm={remove}>
                      {t('admin.feed.editor.delete')}
                    </ConfirmButton>
                  )}
                </Group>
                <Group gap="sm">
                  {published ? (
                    <Button onClick={save} loading={update.isPending} disabled={!valid || !dirty || busy}>
                      {t('admin.feed.editor.saveChanges')}
                    </Button>
                  ) : (
                    <>
                      <Button variant="default" onClick={save} loading={create.isPending || (update.isPending && !publish.isPending)}
                        disabled={!valid || (!dirty && !!post) || busy}>
                        {t('admin.feed.editor.saveDraft')}
                      </Button>
                      <ConfirmButton leftSection={<IconSend size={16} />} disabled={!valid || busy} loading={publish.isPending}
                        message={ids.length > 0 ? t('admin.feed.editor.publishConfirm', { labels: names.join(', ') }) : t('admin.feed.editor.publishConfirmCabinetOnly')}
                        onConfirm={() => void doPublish()}>
                        {t('admin.feed.editor.publish')}
                      </ConfirmButton>
                    </>
                  )}
                </Group>
              </Group>
            </Stack>
          </Card>
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 5 }}>
          <Stack>
            <Stack gap="xs" data-testid="post-preview">
              <Text size="sm" fw={600} c="dimmed">
                {t('admin.feed.editor.preview')}
              </Text>
              {preview.body || preview.imageUrl ? (
                <PostCard post={preview} />
              ) : (
                <Card withBorder>
                  <Text size="sm" c="dimmed">{t('admin.feed.editor.previewEmpty')}</Text>
                </Card>
              )}
            </Stack>
            {published && <Deliveries post={post} />}
          </Stack>
        </Grid.Col>
      </Grid>
    </Stack>
  )
}

/** Loads what the editor needs: the post (or the default labels for a new one), labels and chats. */
function EditorLoader({ id }: { id: number | null }) {
  const { t } = useTranslation()
  const post = useFeedPost(id)
  const settings = useFeedSettings()
  const labels = useLabels()
  const chats = useChats()
  const queries = id === null ? [settings, labels, chats] : [post, labels, chats]
  const failed = queries.find((q) => q.isError)
  if (failed) return <Alert color="red">{errorText(t, failed.error)}</Alert>
  if (queries.some((q) => q.isPending) || !labels.data || !chats.data) return <Loader />
  const reaching = labels.data.filter((l) => reachesChat(l, chats.data))
  if (id === null) {
    const defaults = (settings.data?.defaultLabelIds ?? []).filter((x) => reaching.some((l) => l.id === x))
    return <Editor key="new" initialLabels={defaults} offered={reaching} />
  }
  if (!post.data) return <Loader />
  const own = post.data.labelIds
  const offered = labels.data.filter((l) => reachesChat(l, chats.data) || own.includes(l.id))
  return <Editor key={post.data.id} post={post.data} initialLabels={own} offered={offered} />
}

/** Writing, publishing and editing one feed post. */
export function FeedPostEditor() {
  const { t } = useTranslation()
  const { id } = useParams()
  const n = id === undefined ? null : Number(id)
  return (
    <Stack>
      <BackLink to="/admin/modules/feed">{t('admin.feed.editor.back')}</BackLink>
      <ModuleGate code="feed">
        {n !== null && !Number.isInteger(n) ? <Alert color="red">{t('errors.not_found')}</Alert> : <EditorLoader id={n} />}
      </ModuleGate>
    </Stack>
  )
}
