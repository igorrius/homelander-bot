import { Alert, Button, Checkbox, Group, Modal, Stack, Text, TextInput } from '@mantine/core'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateLabel, useLabels, useSetLabels } from '../../api/chats'
import type { LabelTargetType } from '../../api/types'
import { useApiErrors } from '../../lib/errors'

/** Picks the labels of a chat, topic or person; a new label can be created on the spot. */
export function LabelsModal({ target, current, title, onClose }: {
  target: { type: LabelTargetType; id: number }
  current: number[]
  title: string
  onClose(): void
}) {
  const { t } = useTranslation()
  const labels = useLabels()
  const create = useCreateLabel()
  const save = useSetLabels()
  const [selected, setSelected] = useState<number[]>(current)
  const [name, setName] = useState('')
  const created = useApiErrors(create.error)
  const saved = useApiErrors(save.error)
  const toggle = (id: number, on: boolean) => setSelected((s) => (on ? [...s, id].sort((a, b) => a - b) : s.filter((x) => x !== id)))
  return (
    <Modal opened onClose={onClose} title={title} centered closeButtonProps={{ 'aria-label': t('common.close') }}>
      <Stack>
        {(labels.data ?? []).length === 0 && <Text c="dimmed">{t('admin.labels.empty')}</Text>}
        {(labels.data ?? []).map((l) => (
          <Checkbox key={l.id} label={l.name} description={l.description || undefined} checked={selected.includes(l.id)}
            onChange={(e) => toggle(l.id, e.currentTarget.checked)} />
        ))}
        <Group align="flex-end" wrap="nowrap">
          <TextInput style={{ flex: 1 }} label={t('admin.labels.newLabel')} value={name} error={created.fields.name ?? created.message}
            onChange={(e) => setName(e.currentTarget.value)} />
          <Button variant="default" loading={create.isPending} disabled={name.trim() === ''}
            onClick={() => create.mutate({ name: name.trim(), description: '' }, {
              onSuccess: (l) => {
                toggle(l.id, true)
                setName('')
              },
            })}>
            {t('admin.labels.create')}
          </Button>
        </Group>
        {saved.message && <Alert color="red">{saved.message}</Alert>}
        <Group justify="flex-end">
          <Button loading={save.isPending} onClick={() => save.mutate({ ...target, labelIds: selected }, { onSuccess: onClose })}>
            {t('common.save')}
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
