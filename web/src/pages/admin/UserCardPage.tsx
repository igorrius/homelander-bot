import { ActionIcon, Alert, Anchor, Badge, Breadcrumbs, Button, Card, Grid, Group, Loader, Select, Stack, Table, Tabs, Text, TextInput, Title } from '@mantine/core'
import { IconHistory, IconHome, IconId, IconMessages, IconPencil, IconTrash } from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams, useSearchParams } from 'react-router'
import {
  useAddResidency, useDeleteResidency, useHouses, usePatchUser, useStatuses, useUnlinkChannel, useUpdateResidency, useUserCard,
  type UserPatch,
} from '../../api/admin'
import type { AdminUser, Residency, UserCard } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { supportedLocales } from '../../i18n'
import { errorText, useApiErrors } from '../../lib/errors'
import { formatDateTime, fullName } from '../../lib/format'
import { useLocalized } from '../../lib/localized'
import { AuditTable } from './AuditTable'
import { ResidencyEditor } from './ResidencyEditor'
import { UserSummary } from './UserSummary'

function useSaved() {
  const { t } = useTranslation()
  return () => notifications.show({ message: t('common.saved'), color: 'green' })
}

function Details({ user }: { user: AdminUser }) {
  const { t } = useTranslation()
  const patch = usePatchUser(user.id)
  const saved = useSaved()
  const [firstName, setFirstName] = useState(user.firstName)
  const [lastName, setLastName] = useState(user.lastName)
  const [phone, setPhone] = useState(user.phone)
  const [locale, setLocale] = useState(user.locale)
  const { message, fields } = useApiErrors(patch.error)
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    const body: UserPatch = {}
    if (firstName.trim() !== user.firstName) body.firstName = firstName.trim()
    if (lastName.trim() !== user.lastName) body.lastName = lastName.trim()
    if (phone.trim() !== user.phone) body.phone = phone.trim()
    if (locale !== user.locale) body.locale = locale
    if (Object.keys(body).length > 0) patch.mutate(body, { onSuccess: saved })
  }
  return (
    <Card withBorder>
      <form onSubmit={onSubmit}>
        <Stack>
          <Title order={3} size="h4">{t('admin.user.details')}</Title>
          <Group grow align="flex-start">
            <TextInput label={t('register.firstName')} value={firstName} onChange={(e) => setFirstName(e.currentTarget.value)} error={fields.firstName} />
            <TextInput label={t('register.lastName')} value={lastName} onChange={(e) => setLastName(e.currentTarget.value)} error={fields.lastName} />
          </Group>
          <Group grow align="flex-start">
            <TextInput label={t('register.phone')} type="tel" value={phone} onChange={(e) => setPhone(e.currentTarget.value)} error={fields.phone}
              description={user.hasTelegram ? t('admin.user.phoneLocked') : undefined} />
            <Select label={t('admin.user.locale')} value={locale} onChange={(v) => v && setLocale(v)} allowDeselect={false}
              data={supportedLocales.map((l) => ({ value: l, label: t(`language.${l}`) }))} error={fields.locale} />
          </Group>
          {message && <Alert color="red">{message}</Alert>}
          <Group>
            <Button type="submit" loading={patch.isPending}>
              {t('admin.user.saveDetails')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Card>
  )
}

function Channels({ card }: { card: UserCard }) {
  const { t, i18n } = useTranslation()
  const unlink = useUnlinkChannel(card.user.id)
  const saved = useSaved()
  return (
    <Card withBorder>
      <Stack>
        <Title order={3} size="h4">{t('admin.user.channels')}</Title>
        {card.channels.length === 0 ? (
          <Text c="dimmed">{t('admin.user.noChannels')}</Text>
        ) : (
          card.channels.map((c) => (
            <Group key={c.id} justify="space-between">
              <Group gap="xs">
                <Badge variant="light">{t(`admin.user.channelType.${c.type}`, { defaultValue: c.type })}</Badge>
                {c.username && <Text>{`@${c.username}`}</Text>}
                <Text size="sm" c="dimmed">
                  {formatDateTime(c.linkedAt, i18n.language)}
                </Text>
              </Group>
              {card.user.source !== 'bootstrap' && (
                <ConfirmButton size="xs" color="red" variant="light" loading={unlink.isPending} message={t('admin.user.unlinkHint')}
                  onConfirm={() => unlink.mutate(c.id, { onSuccess: saved })}>
                  {t('admin.user.unlink')}
                </ConfirmButton>
              )}
            </Group>
          ))
        )}
        {unlink.error && <Alert color="red">{errorText(t, unlink.error)}</Alert>}
      </Stack>
    </Card>
  )
}

function Residencies({ card }: { card: UserCard }) {
  const { t } = useTranslation()
  const text = useLocalized()
  const houses = useHouses(true)
  const statuses = useStatuses()
  const add = useAddResidency(card.user.id)
  const update = useUpdateResidency()
  const remove = useDeleteResidency()
  const saved = useSaved()
  const [editing, setEditing] = useState<number | 'new' | null>(null)
  const lists = { houses: houses.data ?? [], statuses: statuses.data ?? [] }
  const done = () => {
    setEditing(null)
    saved()
  }
  const row = (r: Residency) =>
    editing === r.id ? (
      <Table.Tr key={r.id}>
        <Table.Td colSpan={4}>
          <ResidencyEditor {...lists} initial={r} busy={update.isPending} error={update.error}
            onSave={(v) => update.mutate({ id: r.id, ...v }, { onSuccess: done })} onCancel={() => setEditing(null)} />
        </Table.Td>
      </Table.Tr>
    ) : (
      <Table.Tr key={r.id}>
        <Table.Td>{r.houseName}</Table.Td>
        <Table.Td>{r.apartment}</Table.Td>
        <Table.Td>{text(r.statusNames)}</Table.Td>
        <Table.Td>
          <Group gap="xs" justify="flex-end" wrap="nowrap">
            <ActionIcon variant="subtle" aria-label={t('admin.user.editResidency')} onClick={() => {
              update.reset()
              setEditing(r.id)
            }}>
              <IconPencil size={16} />
            </ActionIcon>
            <ConfirmButton size="xs" variant="subtle" color="red" aria-label={t('admin.user.removeResidency')} title={t('admin.user.removeResidency')}
              message={t('admin.user.removeResidencyConfirm', { house: r.houseName, n: r.apartment })} onConfirm={() => remove.mutate(r.id, { onSuccess: saved })}>
              <IconTrash size={16} />
            </ConfirmButton>
          </Group>
        </Table.Td>
      </Table.Tr>
    )
  return (
    <Card withBorder>
      <Stack>
        <Group justify="space-between">
          <Title order={3} size="h4">{t('admin.user.residencies')}</Title>
          {editing !== 'new' && (
            <Button size="xs" variant="light" onClick={() => {
              add.reset()
              setEditing('new')
            }}>
              {t('admin.user.addResidency')}
            </Button>
          )}
        </Group>
        {!card.user.isResident && card.residencies.length > 0 && <Alert color="yellow">{t('admin.user.notResidentHint')}</Alert>}
        <Table aria-label={t('admin.user.residencies')}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('register.house')}</Table.Th>
              <Table.Th>{t('register.apartment')}</Table.Th>
              <Table.Th>{t('admin.user.residencyStatus')}</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>{card.residencies.map(row)}</Table.Tbody>
        </Table>
        {card.residencies.length === 0 && editing !== 'new' && <Text c="dimmed">{t('admin.user.noResidencies')}</Text>}
        {editing === 'new' && (
          <ResidencyEditor {...lists} busy={add.isPending} error={add.error} onSave={(v) => add.mutate(v, { onSuccess: done })} onCancel={() => setEditing(null)} />
        )}
        {remove.error && <Alert color="red">{errorText(t, remove.error)}</Alert>}
      </Stack>
    </Card>
  )
}

const tabs = ['details', 'apartments', 'channels', 'history'] as const
type Tab = (typeof tabs)[number]

function Count({ n }: { n: number }) {
  return (
    <Badge size="xs" variant="light" circle>
      {n}
    </Badge>
  )
}

export function UserCardPage() {
  const { t } = useTranslation()
  const id = Number(useParams().id)
  const card = useUserCard(id)
  const [params, setParams] = useSearchParams()
  const raw = params.get('tab')
  const tab: Tab = (tabs as readonly string[]).includes(raw ?? '') ? (raw as Tab) : 'details'
  const setTab = (v: string | null) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('tab', v ?? 'details')
        return next
      },
      { replace: true },
    )
  if (card.isPending) return <Loader />
  if (card.isError) return <Alert color="red">{errorText(t, card.error)}</Alert>
  const { user } = card.data
  const detailsKey = [user.firstName, user.lastName, user.phone, user.locale].join('|')
  // key: sections keep form and error state; never show them on another user's card.
  return (
    <Stack key={id}>
      <Breadcrumbs>
        <Anchor component={Link} to="/admin/users" size="sm">
          {t('admin.nav.users')}
        </Anchor>
        <Text size="sm" c="dimmed">
          {fullName(user)}
        </Text>
      </Breadcrumbs>
      <Grid gap="md" align="flex-start">
        <Grid.Col span={{ base: 12, md: 4 }}>
          <UserSummary card={card.data} />
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 8 }}>
          <Tabs value={tab} onChange={(v) => setTab(v)}>
            <Tabs.List mb="md" style={{ flexWrap: 'nowrap', overflowX: 'auto' }}>
              <Tabs.Tab value="details" leftSection={<IconId size={16} />}>
                {t('admin.user.tabs.details')}
              </Tabs.Tab>
              <Tabs.Tab value="apartments" leftSection={<IconHome size={16} />} rightSection={<Count n={card.data.residencies.length} />}>
                {t('admin.user.tabs.apartments')}
              </Tabs.Tab>
              <Tabs.Tab value="channels" leftSection={<IconMessages size={16} />} rightSection={<Count n={card.data.channels.length} />}>
                {t('admin.user.tabs.channels')}
              </Tabs.Tab>
              <Tabs.Tab value="history" leftSection={<IconHistory size={16} />}>
                {t('admin.user.tabs.history')}
              </Tabs.Tab>
            </Tabs.List>
            <Tabs.Panel value="details">
              <Details key={detailsKey} user={user} />
            </Tabs.Panel>
            <Tabs.Panel value="apartments">
              <Residencies card={card.data} />
            </Tabs.Panel>
            <Tabs.Panel value="channels">
              <Channels card={card.data} />
            </Tabs.Panel>
            <Tabs.Panel value="history">
              <Card>
                <Stack>
                  <Group justify="space-between">
                    <Title order={3} size="h4">{t('admin.user.audit')}</Title>
                    <Anchor component={Link} to={`/admin/audit?userId=${user.id}`} size="sm">
                      {t('admin.user.allAudit')}
                    </Anchor>
                  </Group>
                  <AuditTable entries={card.data.audit} />
                </Stack>
              </Card>
            </Tabs.Panel>
          </Tabs>
        </Grid.Col>
      </Grid>
    </Stack>
  )
}
