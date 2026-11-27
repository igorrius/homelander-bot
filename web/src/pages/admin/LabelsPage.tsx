import { Alert, Badge, Button, Group, Loader, Modal, Stack, Table, Text, TextInput, Textarea } from '@mantine/core'
import { IconMessages, IconPlus } from '@tabler/icons-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useChats, useCreateLabel, useDeleteLabel, useLabels, useUpdateLabel } from '../../api/chats'
import type { Label } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageHeader } from '../../components/PageHeader'
import { labelPlaces } from '../../lib/chats'
import { errorText, useApiErrors } from '../../lib/errors'
import { ChatsTabs } from './ChatsTabs'
import { TestSendButton } from './TestSendButton'

function LabelForm({ initial, onDone }: { initial?: Label; onDone(): void }) {
  const { t } = useTranslation()
  const create = useCreateLabel()
  const update = useUpdateLabel(initial?.id ?? 0)
  const m = initial ? update : create
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const { message, fields } = useApiErrors(m.error)
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    m.mutate({ name: name.trim(), description: description.trim() }, { onSuccess: onDone })
  }
  return (
    <form onSubmit={onSubmit}>
      <Stack>
        <TextInput label={t('admin.labels.name')} value={name} onChange={(e) => setName(e.currentTarget.value)} error={fields.name} data-autofocus />
        <Textarea label={t('admin.labels.descriptionField')} value={description} onChange={(e) => setDescription(e.currentTarget.value)} error={fields.description} autosize minRows={2} />
        {message && <Alert color="red">{message}</Alert>}
        <Group justify="flex-end">
          <Button type="submit" loading={m.isPending}>{t('common.save')}</Button>
        </Group>
      </Stack>
    </form>
  )
}

/** The list of labels: where each hangs, what uses it. */
export function LabelsPage() {
  const { t } = useTranslation()
  const labels = useLabels()
  const chats = useChats()
  const del = useDeleteLabel()
  const [editing, setEditing] = useState<Label | 'new' | null>(null)
  const places = (l: Label) => labelPlaces(t, l, chats.data ?? [])
  const deleteMessage = (l: Label) =>
    [
      t('admin.labels.deleteConfirm', { name: l.name }),
      l.targets.length > 0 ? t('admin.labels.deleteLoses', { places: places(l).join(', ') }) : '',
      l.usedByApplications ? t('admin.labels.deleteSetting', { settings: t('admin.labels.usedByApplications') }) : '',
    ].filter(Boolean).join(' ')
  return (
    <Stack>
      <PageHeader icon={IconMessages} title={t('admin.chats.title')} description={t('admin.labels.description')}
        actions={<Button leftSection={<IconPlus size={16} />} onClick={() => setEditing('new')}>{t('admin.labels.add')}</Button>} />
      <ChatsTabs />
      {labels.isPending ? (
        <Loader />
      ) : labels.isError ? (
        <Alert color="red">{errorText(t, labels.error)}</Alert>
      ) : labels.data.length === 0 ? (
        <Text c="dimmed">{t('admin.labels.empty')}</Text>
      ) : (
        <Table striped>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('admin.labels.name')}</Table.Th>
              <Table.Th>{t('admin.labels.where')}</Table.Th>
              <Table.Th>{t('admin.labels.usedBy')}</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {labels.data.map((l) => (
              <Table.Tr key={l.id}>
                <Table.Td>
                  <Text fw={500}>{l.name}</Text>
                  {l.description && <Text size="xs" c="dimmed">{l.description}</Text>}
                </Table.Td>
                <Table.Td>
                  {l.targets.length === 0 ? (
                    <Text size="sm" c="dimmed">{t('admin.labels.nowhere')}</Text>
                  ) : (
                    <Group gap={4}>
                      {places(l).map((p) => <Badge key={p} variant="light" radius="sm" tt="none">{p}</Badge>)}
                    </Group>
                  )}
                </Table.Td>
                <Table.Td>{l.usedByApplications && <Badge color="grape" radius="sm" tt="none">{t('admin.labels.usedByApplications')}</Badge>}</Table.Td>
                <Table.Td>
                  <Group gap={4} justify="flex-end" wrap="nowrap">
                    {l.targets.length > 0 && <TestSendButton target={{ type: 'label', id: l.id }} />}
                    <Button size="xs" variant="subtle" onClick={() => setEditing(l)}>{t('common.edit')}</Button>
                    <ConfirmButton size="xs" variant="subtle" color="red" message={deleteMessage(l)} onConfirm={() => del.mutate(l.id)}>
                      {t('common.delete')}
                    </ConfirmButton>
                  </Group>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
      {del.isError && <Alert color="red">{errorText(t, del.error)}</Alert>}
      <Modal opened={editing !== null} onClose={() => setEditing(null)} title={editing === 'new' ? t('admin.labels.add') : t('common.edit')} centered
        closeButtonProps={{ 'aria-label': t('common.close') }}>
        {editing !== null && <LabelForm initial={editing === 'new' ? undefined : editing} onDone={() => setEditing(null)} />}
      </Modal>
    </Stack>
  )
}
