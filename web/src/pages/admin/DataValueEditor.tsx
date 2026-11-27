import { Alert, Button, FileInput, Group, Image, NumberInput, Stack, Switch, Text, Textarea } from '@mantine/core'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { adminAssetUrl, usePatchDataItem, useUploadDataImage } from '../../api/data'
import type { DataItem, LocalizedText } from '../../api/types'
import { LocalizedTextInputs } from '../../components/LocalizedTextInputs'
import { useApiErrors } from '../../lib/errors'

function initialDraft(item: DataItem): unknown {
  const v = item.value ?? null
  switch (item.valueType) {
    case 'text':
      return typeof v === 'string' ? v : ''
    case 'json':
      return v === null ? '' : JSON.stringify(v, null, 2)
    case 'number':
      return typeof v === 'number' ? v : ''
    case 'bool':
      return v === true
    case 'localized_text':
      return (v ?? {}) as LocalizedText
    default:
      return v
  }
}

/** Editor of one value item; images are saved as soon as they are uploaded. */
export function DataValueEditor({ item }: { item: DataItem }) {
  const { t } = useTranslation()
  const patch = usePatchDataItem(item.key)
  const upload = useUploadDataImage()
  const [draft, setDraft] = useState<unknown>(() => initialDraft(item))
  const [jsonError, setJsonError] = useState(false)
  const { message, fields } = useApiErrors(patch.error ?? upload.error)
  const label = t('admin.data.value')
  const vt = item.valueType

  const save = () => {
    let value: unknown = draft
    if (vt === 'json') {
      const text = String(draft).trim()
      if (text === '') value = null
      else {
        try {
          value = JSON.parse(text)
        } catch {
          setJsonError(true)
          return
        }
      }
    } else if (vt === 'text') value = draft === '' ? null : draft
    else if (vt === 'number') value = draft === '' ? null : Number(draft)
    setJsonError(false)
    patch.mutate({ valueType: vt, value })
  }

  if (vt === 'image') {
    const id = typeof item.value === 'number' ? item.value : null
    return (
      <Stack>
        {id ? <Image src={adminAssetUrl(id)} alt={item.key} mah={240} w="auto" fit="contain" radius="sm" /> : <Text c="dimmed">{t('admin.data.noImage')}</Text>}
        <Group align="flex-end">
          <FileInput label={t('admin.data.uploadImage')} accept="image/png,image/jpeg,image/webp,image/gif" value={null} error={fields.file ?? fields.value}
            onChange={(f) => f && upload.mutate(f, { onSuccess: (a) => patch.mutate({ valueType: 'image', value: a.id }) })} />
          {id && (
            <Button variant="default" loading={patch.isPending} onClick={() => patch.mutate({ valueType: 'image', value: null })}>
              {t('admin.data.clearImage')}
            </Button>
          )}
        </Group>
        {message && <Alert color="red">{message}</Alert>}
      </Stack>
    )
  }

  return (
    <Stack>
      {vt === 'text' && <Textarea label={label} value={String(draft)} onChange={(e) => setDraft(e.currentTarget.value)} autosize minRows={2} error={fields.value} />}
      {vt === 'json' && (
        <Textarea label={label} value={String(draft)} onChange={(e) => setDraft(e.currentTarget.value)} autosize minRows={4} ff="monospace"
          error={jsonError ? t('admin.data.invalidJson') : fields.value} />
      )}
      {vt === 'number' && <NumberInput label={label} value={draft as number | ''} onChange={(v) => setDraft(v === '' ? '' : Number(v))} error={fields.value} />}
      {vt === 'bool' && <Switch label={label} checked={draft === true} onChange={(e) => setDraft(e.currentTarget.checked)} />}
      {vt === 'localized_text' && (
        <LocalizedTextInputs label={label} value={draft as LocalizedText} onChange={setDraft} error={fields.value} multiline />
      )}
      {message && <Alert color="red">{message}</Alert>}
      <Group justify="flex-end">
        <Button onClick={save} loading={patch.isPending}>{t('admin.data.saveValue')}</Button>
      </Group>
    </Stack>
  )
}
