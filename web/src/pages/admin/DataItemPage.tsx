import { Alert, Button, Card, Code, Group, Input, Loader, Stack, Text, Textarea } from '@mantine/core'
import { IconDatabase, IconLock } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { BackLink } from '../../components/BackLink'
import { useNavigate, useParams } from 'react-router'
import { publicDataUrl, useDataItem, useDeleteDataItem, usePatchDataItem } from '../../api/data'
import { useModuleName } from '../../api/modules'
import type { DataItem } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageHeader } from '../../components/PageHeader'
import { errorText, useApiErrors } from '../../lib/errors'
import { DataTableEditor } from './DataTableEditor'
import { DataValueEditor } from './DataValueEditor'
import { VisibilityControl } from './VisibilityControl'

function Settings({ item }: { item: DataItem }) {
  const { t } = useTranslation()
  const patch = usePatchDataItem(item.key)
  const [description, setDescription] = useState(item.description)
  const { message, fields } = useApiErrors(patch.error)
  const owned = !!item.owner
  return (
    <Card withBorder>
      <Stack>
        {owned ? (
          <Input.Wrapper label={t('admin.data.descriptionField')}>
            <Text size="sm" c={item.description ? undefined : 'dimmed'} mt={4}>
              {item.description || '—'}
            </Text>
          </Input.Wrapper>
        ) : (
          <>
            <Textarea label={t('admin.data.descriptionField')} description={t('admin.data.descriptionHint')} value={description}
              onChange={(e) => setDescription(e.currentTarget.value)} error={fields.description} autosize minRows={2} />
            <Group justify="flex-end">
              <Button variant="default" loading={patch.isPending} disabled={description.trim() === item.description}
                onClick={() => patch.mutate({ description: description.trim() })}>
                {t('admin.data.saveDescription')}
              </Button>
            </Group>
          </>
        )}
        <VisibilityControl value={item.visibility} disabled={patch.isPending} onChange={(visibility) => patch.mutate({ visibility })} />
        {item.visibility === 'public' && (
          <Text size="sm">
            {t('admin.data.publicUrl')} <Code>{publicDataUrl(item.key)}</Code>
          </Text>
        )}
        {message && <Alert color="red">{message}</Alert>}
      </Stack>
    </Card>
  )
}

/** Says which module writes an item, and that its data is not served while the module is off. */
function OwnedNote({ item }: { item: DataItem }) {
  const { t } = useTranslation()
  const name = useModuleName(item.owner ?? '')
  return (
    <Alert color="grape" icon={<IconLock size={18} />}>
      <Stack gap={4}>
        <Text size="sm">{t('admin.data.ownedAlert', { name })}</Text>
        {item.ownerEnabled === false && <Text size="sm" c="dimmed">{t('admin.data.ownedOffAlert')}</Text>}
      </Stack>
    </Alert>
  )
}

/** One value or table: its settings and its contents. A module's item: only who sees it changes here. */
export function DataItemPage() {
  const { t } = useTranslation()
  const { key = '' } = useParams()
  const navigate = useNavigate()
  const q = useDataItem(key)
  const del = useDeleteDataItem()
  if (q.isPending) return <Loader />
  if (q.isError) return <Alert color="red">{errorText(t, q.error)}</Alert>
  const { item, rows } = q.data
  const owned = !!item.owner
  const kind = item.kind === 'value' ? `${t('admin.data.kind.value')} · ${t(`admin.data.type.${item.valueType}`)}` : t('admin.data.kind.table')
  return (
    <Stack>
      <BackLink to="/admin/data">{t('admin.data.back')}</BackLink>
      <PageHeader icon={IconDatabase} title={item.key} description={kind}
        actions={!owned && (
          <ConfirmButton color="red" variant="light" message={t('admin.data.deleteItemConfirm', { key: item.key })}
            onConfirm={() => del.mutate(item.key, { onSuccess: () => navigate('/admin/data') })}>
            {t('admin.data.deleteItem')}
          </ConfirmButton>
        )} />
      {owned && <OwnedNote item={item} />}
      <Settings key={`s-${q.dataUpdatedAt}`} item={item} />
      <Card withBorder>
        {item.kind === 'value' ? (
          <DataValueEditor key={`v-${q.dataUpdatedAt}`} item={item} readOnly={owned} />
        ) : (
          <DataTableEditor key={`t-${q.dataUpdatedAt}`} item={item} rows={rows} readOnly={owned} />
        )}
      </Card>
      {del.error && <Alert color="red">{errorText(t, del.error)}</Alert>}
    </Stack>
  )
}
