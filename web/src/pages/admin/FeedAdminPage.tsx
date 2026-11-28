import { Alert, Anchor, Badge, Button, Card, Divider, Group, Loader, MultiSelect, Pagination, Stack, Tabs, Text } from '@mantine/core'
import { IconList, IconNews, IconPhoto, IconPlus, IconSettings } from '@tabler/icons-react'
import { Fragment, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { useChats, useLabels } from '../../api/chats'
import { useFeedPosts, useFeedSettings, useSaveFeedSettings } from '../../api/feed'
import { useModuleEnabled } from '../../api/modules'
import type { AdminPost } from '../../api/types'
import { EmptyState } from '../../components/EmptyState'
import { PageHeader } from '../../components/PageHeader'
import { reachesChat } from '../../lib/chats'
import { errorText, useApiErrors } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import { ModuleGate } from './ModuleOff'

type Tab = 'posts' | 'settings'
const pageSize = 20

/** The first line or so of a post for a list. */
function excerpt(body: string): string {
  const s = body.replace(/\s+/g, ' ').trim()
  return s.length > 160 ? `${s.slice(0, 159)}…` : s
}

/** Draft or Published as a badge. */
export function PostStateBadge({ post }: { post: AdminPost }) {
  const { t } = useTranslation()
  return (
    <Badge variant="light" color={post.state === 'published' ? 'teal' : 'gray'}>
      {t(`admin.feed.state.${post.state}`)}
    </Badge>
  )
}

/** Sent / queued / failed counts of a published post, or "cabinet only". */
function DeliverySummary({ post }: { post: AdminPost }) {
  const { t } = useTranslation()
  if (post.state !== 'published') return null
  if (post.labelIds.length === 0) {
    return (
      <Badge variant="outline" color="gray">
        {t('admin.feed.delivery.cabinetOnly')}
      </Badge>
    )
  }
  const { sent, pending, failed } = post.delivery
  return (
    <>
      {(sent > 0 || (!pending && !failed && !post.deliveryExpired)) && (
        <Badge variant="dot" color="teal">
          {t('admin.feed.delivery.sent', { count: sent })}
        </Badge>
      )}
      {pending > 0 && (
        <Badge variant="dot" color="yellow">
          {t('admin.feed.delivery.pending', { count: pending })}
        </Badge>
      )}
      {failed > 0 && (
        <Badge variant="dot" color="red">
          {t('admin.feed.delivery.failed', { count: failed })}
        </Badge>
      )}
    </>
  )
}

function PostRow({ post }: { post: AdminPost }) {
  const { t, i18n } = useTranslation()
  const when =
    post.state === 'published'
      ? t('admin.feed.publishedAt', { date: formatDateTime(post.publishedAt, i18n.language) })
      : t('admin.feed.createdAt', { date: formatDateTime(post.createdAt, i18n.language) })
  return (
    <Group justify="space-between" align="flex-start" gap="sm" p="md" data-testid={`post-${post.id}`}>
      <Stack gap={4} style={{ flex: '1 1 260px', minWidth: 0 }}>
        <Anchor component={Link} to={`/admin/modules/feed/posts/${post.id}`} fw={500} c="inherit" lineClamp={2}>
          {excerpt(post.body) || t('admin.feed.noText')}
        </Anchor>
        <Group gap={6} wrap="nowrap">
          {post.imageAssetId !== null && <IconPhoto size={14} stroke={1.6} color="var(--mantine-color-dimmed)" aria-hidden />}
          <Text size="xs" c="dimmed">
            {when}
            {post.createdBy && ` · ${post.createdBy.name}`}
          </Text>
        </Group>
      </Stack>
      <Group gap={6} justify="flex-end">
        <PostStateBadge post={post} />
        <DeliverySummary post={post} />
      </Group>
    </Group>
  )
}

function PostsTab() {
  const { t } = useTranslation()
  const [params, setParams] = useSearchParams()
  const page = Number(params.get('page')) || 1
  const posts = useFeedPosts(page, pageSize)
  const total = posts.data?.total ?? 0
  const setPage = (p: number) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('page', String(p))
      return next
    })
  if (posts.isPending) return <Loader />
  if (posts.isError) return <Alert color="red">{errorText(t, posts.error)}</Alert>
  if (posts.data.items.length === 0) {
    return (
      <Card withBorder>
        <EmptyState icon={IconNews} title={t('admin.feed.empty')} hint={t('admin.feed.emptyHint')} />
      </Card>
    )
  }
  return (
    <Stack>
      <Card withBorder p={0}>
        {posts.data.items.map((p, i) => (
          <Fragment key={p.id}>
            {i > 0 && <Divider />}
            <PostRow post={p} />
          </Fragment>
        ))}
      </Card>
      {total > pageSize && <Pagination total={Math.ceil(total / pageSize)} value={page} onChange={setPage} />}
    </Stack>
  )
}

function SettingsTab() {
  const { t } = useTranslation()
  const settings = useFeedSettings()
  const labels = useLabels()
  const chats = useChats()
  const save = useSaveFeedSettings()
  // The admin's unsaved choice; undefined shows the saved one.
  const [draft, setDraft] = useState<string[] | undefined>(undefined)
  const { message, fields } = useApiErrors(save.error)
  const error = settings.error ?? labels.error ?? chats.error
  if (error) return <Alert color="red">{errorText(t, error)}</Alert>
  if (!settings.data || !labels.data || !chats.data) return <Loader />
  const value = draft ?? settings.data.defaultLabelIds.map(String)
  const allChats = chats.data
  // Labels that reach a chat, and any saved one that no longer does (so it can be removed).
  const offered = labels.data.filter((l) => reachesChat(l, allChats) || value.includes(String(l.id)))
  return (
    <Card withBorder padding="md" maw={560}>
      <Stack>
        <MultiSelect label={t('admin.feed.settings.defaultLabels')} description={t('admin.feed.settings.defaultLabelsHint')}
          placeholder={value.length === 0 ? t('admin.feed.editor.labelsPlaceholder') : undefined}
          data={offered.map((l) => ({ value: String(l.id), label: l.name }))} value={value}
          onChange={(v) => {
            save.reset()
            setDraft(v)
          }}
          error={fields.defaultLabelIds} clearable searchable={offered.length > 8} />
        {offered.length === 0 && <Alert color="orange" p="xs">{t('admin.feed.settings.noLabels')}</Alert>}
        {message && <Alert color="red">{message}</Alert>}
        {save.isSuccess && <Text size="sm" c="teal">{t('common.saved')}</Text>}
        <Group justify="flex-end">
          <Button loading={save.isPending} onClick={() => save.mutate({ defaultLabelIds: value.map(Number) }, { onSuccess: () => setDraft(undefined) })}>
            {t('common.save')}
          </Button>
        </Group>
      </Stack>
    </Card>
  )
}

/** The feed module's admin page: posts and settings, or the turned-off card. */
export function FeedAdminPage() {
  const { t } = useTranslation()
  const [params, setParams] = useSearchParams()
  const tab: Tab = params.get('tab') === 'settings' ? 'settings' : 'posts'
  const on = useModuleEnabled('feed') === true
  return (
    <Stack>
      <PageHeader icon={IconNews} title={t('admin.feed.title')} description={t('admin.feed.description')}
        actions={on && tab === 'posts' && (
          <Button component={Link} to="/admin/modules/feed/posts/new" leftSection={<IconPlus size={16} />}>
            {t('admin.feed.newPost')}
          </Button>
        )} />
      <ModuleGate code="feed">
        <Tabs value={tab} onChange={(v) => setParams({ tab: v ?? 'posts' }, { replace: true })} keepMounted={false}>
          <Tabs.List mb="lg">
            <Tabs.Tab value="posts" leftSection={<IconList size={16} />}>{t('admin.feed.tabs.posts')}</Tabs.Tab>
            <Tabs.Tab value="settings" leftSection={<IconSettings size={16} />}>{t('admin.feed.tabs.settings')}</Tabs.Tab>
          </Tabs.List>
          <Tabs.Panel value="posts">
            <PostsTab />
          </Tabs.Panel>
          <Tabs.Panel value="settings">
            <SettingsTab />
          </Tabs.Panel>
        </Tabs>
      </ModuleGate>
    </Stack>
  )
}
