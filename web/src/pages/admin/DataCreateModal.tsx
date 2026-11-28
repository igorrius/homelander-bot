import { Alert, Button, Group, Modal, Radio, Select, Stack, TextInput, Textarea } from '@mantine/core'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useCreateDataItem, type DataInput } from '../../api/data'
import { valueTypes, type DataKind, type DataType, type Visibility } from '../../api/types'
import { useApiErrors } from '../../lib/errors'
import { VisibilityControl } from './VisibilityControl'

/** Dialog that creates a value or a table and opens it. */
export function DataCreateModal({ opened, onClose }: { opened: boolean; onClose(): void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const create = useCreateDataItem()
  const [key, setKey] = useState('')
  const [kind, setKind] = useState<DataKind>('value')
  const [valueType, setValueType] = useState<DataType>('text')
  const [description, setDescription] = useState('')
  const [visibility, setVisibility] = useState<Visibility>('admins')
  const { message, fields } = useApiErrors(create.error)
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    const body: DataInput =
      kind === 'value'
        ? { key: key.trim(), kind, valueType, description: description.trim(), visibility }
        : { key: key.trim(), kind, columns: [{ name: 'name', label: t('admin.data.firstColumn'), type: 'text' }], description: description.trim(), visibility }
    create.mutate(body, { onSuccess: (item) => navigate(`/admin/data/${item.key}`) })
  }
  return (
    <Modal opened={opened} onClose={onClose} title={t('admin.data.add')}>
      <form onSubmit={onSubmit}>
        <Stack>
          <TextInput label={t('admin.data.key')} description={t('admin.data.keyHint')} value={key} onChange={(e) => setKey(e.currentTarget.value)}
            error={fields.key} data-autofocus />
          <Radio.Group label={t('admin.data.kindLabel')} value={kind} onChange={(v) => setKind(v as DataKind)} error={fields.kind}>
            <Group mt={4}>
              <Radio value="value" label={t('admin.data.kind.value')} />
              <Radio value="table" label={t('admin.data.kind.table')} />
            </Group>
          </Radio.Group>
          {kind === 'value' && (
            <Select label={t('admin.data.valueType')} value={valueType} onChange={(v) => v && setValueType(v as DataType)} allowDeselect={false}
              data={valueTypes.map((v) => ({ value: v, label: t(`admin.data.type.${v}`) }))} error={fields.valueType} />
          )}
          <Textarea label={t('admin.data.descriptionField')} description={t('admin.data.descriptionHint')} value={description}
            onChange={(e) => setDescription(e.currentTarget.value)} error={fields.description} autosize minRows={2} />
          <VisibilityControl value={visibility} onChange={setVisibility} />
          {message && <Alert color="red">{message}</Alert>}
          <Group justify="flex-end">
            <Button type="submit" loading={create.isPending}>{t('admin.data.create')}</Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}
