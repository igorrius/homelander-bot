import { Alert, Anchor, Button, Card, Code, Group, Loader, Stack, Switch, Text, Textarea } from '@mantine/core'
import { IconDatabase } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { publicDataUrl, useDataItem, useDeleteDataItem, usePatchDataItem } from '../../api/data'
import type { DataItem } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageHeader } from '../../components/PageHeader'
import { errorText, useApiErrors } from '../../lib/errors'
import { DataTableEditor } from './DataTableEditor'
import { DataValueEditor } from './DataValueEditor'

function Settings({ item }: { item: DataItem }) {
  const { t } = useTranslation()
  const patch = usePatchDataItem(item.key)
  const [description, setDescription] = useState(item.description)
  const { message, fields } = useApiErrors(patch.error)
  return (
    <Card withBorder>
      <Stack>
        <Textarea label={t('admin.data.descriptionField')} description={t('admin.data.descriptionHint')} value={description}
          onChange={(e) => setDescription(e.currentTarget.value)} error={fields.description} autosize minRows={2} />
        <Group justify="space-between">
          <Switch label={t('admin.data.publicSwitch')} checked={item.public} disabled={patch.isPending}
            onChange={(e) => patch.mutate({ public: e.currentTarget.checked })} />
          <Button variant="default" loading={patch.isPending} disabled={description.trim() === item.description}
            onClick={() => patch.mutate({ description: description.trim() })}>
            {t('admin.data.saveDescription')}
          </Button>
        </Group>
        {item.public && (
          <Text size="sm">
            {t('admin.data.publicUrl')} <Code>{publicDataUrl(item.key)}</Code>
          </Text>
        )}
        {message && <Alert color="red">{message}</Alert>}
      </Stack>
    </Card>
  )
}

/** One value or table: its settings and its contents. */
export function DataItemPage() {
  const { t } = useTranslation()
  const { key = '' } = useParams()
  const navigate = useNavigate()
  const q = useDataItem(key)
  const del = useDeleteDataItem()
  if (q.isPending) return <Loader />
  if (q.isError) return <Alert color="red">{errorText(t, q.error)}</Alert>
  const { item, rows } = q.data
  const kind = item.kind === 'value' ? `${t('admin.data.kind.value')} · ${t(`admin.data.type.${item.valueType}`)}` : t('admin.data.kind.table')
  return (
    <Stack>
      <Anchor component={Link} to="/admin/data" size="sm">{t('admin.data.back')}</Anchor>
      <PageHeader icon={IconDatabase} title={item.key} description={kind}
        actions={
          <ConfirmButton color="red" variant="light" message={t('admin.data.deleteItemConfirm', { key: item.key })}
            onConfirm={() => del.mutate(item.key, { onSuccess: () => navigate('/admin/data') })}>
            {t('admin.data.deleteItem')}
          </ConfirmButton>
        } />
      <Settings key={`s-${q.dataUpdatedAt}`} item={item} />
      <Card withBorder>
        {item.kind === 'value' ? (
          <DataValueEditor key={`v-${q.dataUpdatedAt}`} item={item} />
        ) : (
          <DataTableEditor key={`t-${q.dataUpdatedAt}`} item={item} rows={rows} />
        )}
      </Card>
      {del.error && <Alert color="red">{errorText(t, del.error)}</Alert>}
    </Stack>
  )
}
