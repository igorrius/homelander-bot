import { Alert, Anchor, Badge, Button, Card, Group, Loader, Modal, Stack, Text, TextInput, Title } from '@mantine/core'
import { IconAlertTriangle, IconChevronDown, IconChevronRight, IconMessages } from '@tabler/icons-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useChatAction, useChats, useLabels, useRenameTopic } from '../../api/chats'
import type { Chat, Label, LabelTargetType, Topic } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageHeader } from '../../components/PageHeader'
import { chatKind, topicTitle } from '../../lib/chats'
import { errorText, useApiErrors } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import { ChatsTabs } from './ChatsTabs'
import { LabelsModal } from './LabelsModal'
import { TestSendButton } from './TestSendButton'

const setupGuideURL = 'https://github.com/igorrius/homelander-bot/blob/main/docs/telegram-bot-setup.md'

const stateColor = { new: 'yellow', accepted: 'teal', left: 'gray' } as const

type Editing = { type: LabelTargetType; id: number; current: number[]; title: string }

function LabelBadges({ ids, labels }: { ids: number[]; labels: Label[] }) {
  const { t } = useTranslation()
  if (ids.length === 0) return <Text size="xs" c="dimmed">{t('admin.chats.noLabels')}</Text>
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

function RenameTopic({ topic, onClose }: { topic: Topic; onClose(): void }) {
  const { t } = useTranslation()
  const rename = useRenameTopic()
  const [title, setTitle] = useState(topic.title)
  const { message, fields } = useApiErrors(rename.error)
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    rename.mutate({ id: topic.id, title }, { onSuccess: onClose })
  }
  return (
    <Modal opened onClose={onClose} title={t('admin.chats.rename')} centered closeButtonProps={{ 'aria-label': t('common.close') }}>
      <form onSubmit={onSubmit}>
        <Stack>
          <TextInput label={t('admin.chats.topicTitle')} value={title} onChange={(e) => setTitle(e.currentTarget.value)} error={fields.title} data-autofocus />
          {message && <Alert color="red">{message}</Alert>}
          <Group justify="flex-end">
            <Button type="submit" loading={rename.isPending}>{t('common.save')}</Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}

function ChatCard({ chat, labels, onLabels, onRename }: { chat: Chat; labels: Label[]; onLabels(e: Editing): void; onRename(t: Topic): void }) {
  const { t, i18n } = useTranslation()
  const action = useChatAction()
  const left = chat.state === 'left'
  return (
    <Card withBorder padding="md" data-chat data-testid="chat-card">
      <Stack gap="sm">
        <Group justify="space-between" align="flex-start">
          <Stack gap={4}>
            <Group gap="xs">
              <Text fw={600}>{chat.title || chat.addedBy.externalId}</Text>
              <Badge variant="outline" color="gray" radius="sm" tt="none">{chatKind(t, chat)}</Badge>
              <Badge color={stateColor[chat.state]} radius="sm" tt="none">{t(`admin.chats.state.${chat.state}`)}</Badge>
              {!left && <Badge variant="light" color="gray" radius="sm" tt="none">{t(`admin.chats.bot.${chat.botStatus}`)}</Badge>}
            </Group>
            {chat.addedBy.externalId && chat.addedBy.userId === null && (
              <Text size="xs" c="dimmed">{t('admin.chats.addedBy', { id: chat.addedBy.externalId })}</Text>
            )}
          </Stack>
          <Group gap="xs">
            {chat.state === 'accepted' && <TestSendButton target={{ type: 'chat', id: chat.id }} />}
            {chat.state === 'new' && (
              <Button size="xs" loading={action.isPending} onClick={() => action.mutate({ id: chat.id, action: 'accept' })}>
                {t('admin.chats.accept')}
              </Button>
            )}
            {!left && (
              <ConfirmButton size="xs" variant="default" color="red" message={t('admin.chats.leaveConfirm', { title: chat.title })}
                onConfirm={() => action.mutate({ id: chat.id, action: 'leave' })}>
                {t('admin.chats.leave')}
              </ConfirmButton>
            )}
          </Group>
        </Group>
        {chat.state === 'new' && chat.leaveAt && (
          <Text size="sm" c="orange">{t('admin.chats.leaveSoon', { time: formatDateTime(chat.leaveAt, i18n.language) })}</Text>
        )}
        {left && <Text size="sm" c="dimmed">{t('admin.chats.leftWarning')}</Text>}
        {chat.state !== 'left' && !chat.canSend && chat.botStatus !== 'unknown' && (
          <Alert color="orange" icon={<IconAlertTriangle size={16} />} p="xs">{t('admin.chats.cannotSend')}</Alert>
        )}
        {action.isError && <Alert color="red" p="xs">{errorText(t, action.error)}</Alert>}
        <Group gap="xs">
          <LabelBadges ids={chat.labelIds} labels={labels} />
          <Button size="xs" variant="subtle" onClick={() => onLabels({ type: 'chat', id: chat.id, current: chat.labelIds, title: chat.title })}>
            {t('admin.chats.editLabels')}
          </Button>
        </Group>
        {chat.topics.length > 0 && (
          <Stack gap={6} pl="md" style={{ borderLeft: '2px solid var(--mantine-color-gray-3)' }}>
            <Text size="xs" fw={600} c="dimmed" tt="uppercase">{t('admin.chats.topics')}</Text>
            {chat.topics.map((tp) => (
              <Group key={tp.id} justify="space-between" data-topic>
                <Group gap="xs">
                  <Text size="sm" c={tp.title ? undefined : 'dimmed'}>{topicTitle(t, tp)}</Text>
                  {tp.gone && <Badge size="xs" color="gray">{t('admin.chats.gone')}</Badge>}
                  <LabelBadges ids={tp.labelIds} labels={labels} />
                </Group>
                <Group gap={4}>
                  <Button size="xs" variant="subtle" onClick={() => onLabels({ type: 'chat_topic', id: tp.id, current: tp.labelIds, title: `${chat.title} › ${topicTitle(t, tp)}` })}>
                    {t('admin.chats.editLabels')}
                  </Button>
                  <Button size="xs" variant="subtle" onClick={() => onRename(tp)}>{t('admin.chats.rename')}</Button>
                  {chat.state === 'accepted' && !tp.gone && <TestSendButton target={{ type: 'topic', id: tp.id }} />}
                </Group>
              </Group>
            ))}
          </Stack>
        )}
      </Stack>
    </Card>
  )
}

/** Every chat and topic the bot is in, with acceptance and labels. */
export function ChatsPage() {
  const { t } = useTranslation()
  const chats = useChats()
  const labels = useLabels()
  const [editing, setEditing] = useState<Editing | null>(null)
  const [renaming, setRenaming] = useState<Topic | null>(null)
  const [showLeft, setShowLeft] = useState(false)
  const list = chats.data ?? []
  const present = list.filter((c) => c.state !== 'left')
  const left = list.filter((c) => c.state === 'left')
  const hasNew = list.some((c) => c.state === 'new')
  const card = (c: Chat) => <ChatCard key={c.id} chat={c} labels={labels.data ?? []} onLabels={setEditing} onRename={setRenaming} />
  return (
    <Stack>
      <PageHeader icon={IconMessages} title={t('admin.chats.title')} description={t('admin.chats.description')} />
      <ChatsTabs />
      {chats.isPending ? (
        <Loader />
      ) : chats.isError ? (
        <Alert color="red">{errorText(t, chats.error)}</Alert>
      ) : list.length === 0 ? (
        <Card withBorder padding="lg">
          <Title order={3} size="h5">{t('admin.chats.emptyTitle')}</Title>
          <Text size="sm" c="dimmed" mt={4}>{t('admin.chats.emptyText')}</Text>
          <Anchor href={setupGuideURL} target="_blank" rel="noreferrer" size="sm" mt="sm">{t('admin.chats.setupGuide')}</Anchor>
        </Card>
      ) : (
        <>
          {hasNew && <Alert color="yellow" p="sm">{t('admin.chats.newHint')}</Alert>}
          {present.map(card)}
          {left.length > 0 && (
            <>
              <Button variant="subtle" color="gray" justify="flex-start" onClick={() => setShowLeft((v) => !v)}
                leftSection={showLeft ? <IconChevronDown size={16} /> : <IconChevronRight size={16} />}>
                {`${t('admin.chats.leftChats')} (${left.length})`}
              </Button>
              {showLeft && left.map(card)}
            </>
          )}
        </>
      )}
      {editing && <LabelsModal target={editing} current={editing.current} title={`${t('admin.chats.editLabels')}: ${editing.title}`} onClose={() => setEditing(null)} />}
      {renaming && <RenameTopic topic={renaming} onClose={() => setRenaming(null)} />}
    </Stack>
  )
}
