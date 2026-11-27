import { Alert, Anchor, Badge, Button, Loader, Stack, Table, Text } from '@mantine/core'
import { IconDatabase, IconPlus } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useDataItems } from '../../api/data'
import { PageHeader } from '../../components/PageHeader'
import { errorText } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import { DataCreateModal } from './DataCreateModal'

/** The data space: values and tables the site and, later, modules use. */
export function DataPage() {
  const { t, i18n } = useTranslation()
  const items = useDataItems()
  const [creating, setCreating] = useState(false)
  return (
    <Stack>
      <PageHeader icon={IconDatabase} title={t('admin.data.title')} description={t('admin.data.description')}
        actions={<Button leftSection={<IconPlus size={16} />} onClick={() => setCreating(true)}>{t('admin.data.add')}</Button>} />
      {items.isPending ? (
        <Loader />
      ) : items.isError ? (
        <Alert color="red">{errorText(t, items.error)}</Alert>
      ) : items.data.length === 0 ? (
        <Text c="dimmed">{t('admin.data.empty')}</Text>
      ) : (
        <Table striped>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('admin.data.key')}</Table.Th>
              <Table.Th>{t('admin.data.kindLabel')}</Table.Th>
              <Table.Th>{t('admin.data.access')}</Table.Th>
              <Table.Th>{t('admin.data.rows')}</Table.Th>
              <Table.Th>{t('admin.data.updated')}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {items.data.map((it) => (
              <Table.Tr key={it.key}>
                <Table.Td>
                  <Anchor component={Link} to={`/admin/data/${it.key}`} ff="monospace">{it.key}</Anchor>
                  {it.description && <Text size="xs" c="dimmed">{it.description}</Text>}
                </Table.Td>
                <Table.Td>
                  {t(`admin.data.kind.${it.kind}`)}
                  {it.valueType && <Text size="xs" c="dimmed">{t(`admin.data.type.${it.valueType}`)}</Text>}
                </Table.Td>
                <Table.Td>
                  <Badge variant="light" color={it.public ? 'green' : 'gray'}>{it.public ? t('admin.data.public') : t('admin.data.private')}</Badge>
                </Table.Td>
                <Table.Td>{it.kind === 'table' ? it.rowCount : ''}</Table.Td>
                <Table.Td>
                  <Text size="sm">{formatDateTime(it.updatedAt, i18n.language)}</Text>
                  {it.updatedBy && <Text size="xs" c="dimmed">{it.updatedBy.name}</Text>}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
      <DataCreateModal opened={creating} onClose={() => setCreating(false)} />
    </Stack>
  )
}
