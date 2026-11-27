import { ActionIcon, Alert, Anchor, Avatar, Badge, Button, Card, CopyButton, Divider, Group, Paper, Stack, Switch, Text, Title, Tooltip } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconBrandTelegram, IconCheck, IconCopy, IconPhone } from '@tabler/icons-react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { usePatchUser, useUserAction, type UserAction, type UserPatch } from '../../api/admin'
import { useSite } from '../../api/public'
import type { UserCard } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { errorText } from '../../lib/errors'
import { formatDateTime, fullName } from '../../lib/format'
import { StatusBadge } from './StatusBadge'

function Copy({ value, label }: { value: string; label: string }) {
  const { t } = useTranslation()
  return (
    <CopyButton value={value}>
      {({ copied, copy }) => (
        <Tooltip label={copied ? t('common.copied') : label} withArrow>
          <ActionIcon variant="subtle" size="sm" onClick={copy} aria-label={label} color={copied ? 'green' : 'gray'}>
            {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
          </ActionIcon>
        </Tooltip>
      )}
    </CopyButton>
  )
}

function initials(first: string, last: string): string {
  return `${first.charAt(0)}${last.charAt(0)}`.trim().toUpperCase() || '·'
}

/** For a person approved but not yet activated: the bot link to send them, with its QR code. */
function BotLink() {
  const { t } = useTranslation()
  const site = useSite()
  const url = site.data?.botUrl
  if (!url) return null
  return (
    <Paper withBorder p="sm" bg="var(--mantine-primary-color-light)">
      <Stack gap="xs">
        <Text fw={600} size="sm">
          {t('admin.user.botLink.title')}
        </Text>
        <Text size="sm" c="dimmed">
          {t('admin.user.botLink.hint')}
        </Text>
        <Group gap={4} wrap="nowrap">
          <Anchor href={url} target="_blank" rel="noopener noreferrer" size="sm" style={{ wordBreak: 'break-all' }}>
            {url}
          </Anchor>
          <Copy value={url} label={t('admin.user.copyLink')} />
        </Group>
        <Paper p={4} bg="white" w="fit-content" style={{ lineHeight: 0 }}>
          <QRCodeSVG value={url} size={140} title={t('qr.title')} marginSize={1} />
        </Paper>
      </Stack>
    </Paper>
  )
}

/** The left column of the user card: who this is, how to reach them, roles and the main actions. */
export function UserSummary({ card }: { card: UserCard }) {
  const { t, i18n } = useTranslation()
  const { user } = card
  const patch = usePatchUser(user.id)
  const action = useUserAction()
  const busy = patch.isPending || action.isPending
  const saved = () => notifications.show({ message: t('common.saved'), color: 'green' })
  const run = (a: UserAction) => {
    patch.reset()
    action.mutate({ id: user.id, action: a }, { onSuccess: saved })
  }
  const setFlag = (body: UserPatch) => {
    action.reset()
    patch.mutate(body, { onSuccess: saved })
  }
  const err = patch.error ?? action.error
  const telegram = card.channels.find((c) => c.type === 'telegram')
  const firstAdmin = user.source === 'bootstrap'
  return (
    <Card>
      <Stack gap="md">
        <Group gap="md" wrap="nowrap" align="flex-start">
          <Avatar size={56} radius="xl" color="teal">
            {initials(user.firstName, user.lastName)}
          </Avatar>
          <Stack gap={6} style={{ minWidth: 0 }}>
            <Title order={2} size="h3" style={{ overflowWrap: 'anywhere' }}>
              {fullName(user)}
            </Title>
            <Group gap={6}>
              <StatusBadge status={user.status} />
              <Badge variant="outline" color="gray">
                {t(`admin.user.source.${user.source}`)}
              </Badge>
            </Group>
          </Stack>
        </Group>

        <Stack gap={6}>
          <Group gap="xs" wrap="nowrap">
            <IconPhone size={16} aria-hidden />
            <Text size="sm">{user.phone}</Text>
            <Copy value={user.phone} label={t('admin.user.copyPhone')} />
          </Group>
          <Group gap="xs" wrap="nowrap">
            <IconBrandTelegram size={16} aria-hidden />
            {telegram ? (
              <Text size="sm">{telegram.username ? `@${telegram.username}` : t('admin.user.channelType.telegram')}</Text>
            ) : (
              <Text size="sm" c="dimmed">
                {t('admin.user.telegramNone')}
              </Text>
            )}
          </Group>
        </Stack>

        {user.status === 'awaiting_activation' && <BotLink />}

        <Divider />
        <Stack gap="xs">
          <Text size="xs" fw={600} c="dimmed" tt="uppercase">
            {t('admin.user.roles')}
          </Text>
          <Switch label={t('admin.user.isResident')} checked={user.isResident} disabled={busy} onChange={(e) => setFlag({ isResident: e.currentTarget.checked })} />
          <Switch label={t('admin.user.isAdmin')} checked={user.isAdmin} disabled={busy || firstAdmin} onChange={(e) => setFlag({ isAdmin: e.currentTarget.checked })} />
          <Switch label={t('admin.user.isMcp')} checked={user.isAdmin && !!user.isMcp}
            disabled={busy || !user.isAdmin} onChange={(e) => setFlag({ isMcp: e.currentTarget.checked })} />
        </Stack>

        <Text size="xs" c="dimmed">
          {t('admin.user.created', { at: formatDateTime(user.createdAt, i18n.language) })}
          {user.approvedAt && ` · ${t('admin.user.approved', { at: formatDateTime(user.approvedAt, i18n.language) })}`}
          {user.activatedAt && ` · ${t('admin.user.activated', { at: formatDateTime(user.activatedAt, i18n.language) })}`}
        </Text>

        <Group gap="xs">
          {user.status === 'pending_approval' && (
            <>
              <Button color="green" leftSection={<IconCheck size={16} />} loading={busy} onClick={() => run('approve')}>
                {t('admin.user.approve')}
              </Button>
              <ConfirmButton color="red" variant="light" disabled={busy} message={t('admin.applications.rejectConfirm', { name: fullName(user) })} onConfirm={() => run('reject')}>
                {t('admin.user.reject')}
              </ConfirmButton>
            </>
          )}
          {!firstAdmin && (user.status === 'active' || user.status === 'awaiting_activation') && (
            <ConfirmButton color="red" variant="light" disabled={busy} message={t('admin.user.disableConfirm', { name: fullName(user) })} onConfirm={() => run('disable')}>
              {t('admin.user.disable')}
            </ConfirmButton>
          )}
          {user.status === 'disabled' && (
            <Button loading={busy} onClick={() => run('enable')}>
              {t('admin.user.enable')}
            </Button>
          )}
        </Group>
        {firstAdmin && (
          <Text size="xs" c="dimmed">
            {t('admin.user.firstAdminLocked')}
          </Text>
        )}
        {err && <Alert color="red">{errorText(t, err)}</Alert>}
      </Stack>
    </Card>
  )
}
