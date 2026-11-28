import { Alert, Anchor, Badge, Button, Group, Loader, SegmentedControl, Stack, Table, Text } from '@mantine/core'
import { IconDatabase, IconPlus, IconPuzzle } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useDataItems } from '../../api/data'
import { useModuleName } from '../../api/modules'
import { visibilities, type DataItem, type Visibility } from '../../api/types'
import { PageHeader } from '../../components/PageHeader'
import { errorText } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import { DataCreateModal } from './DataCreateModal'
import { VisibilityBadge } from './VisibilityControl'

/** "Module: News feed", and "module off" while that module is turned off. */
function OwnerBadges({ item }: { item: DataItem }) {
  const { t } = useTranslation()
  const name = useModuleName(item.owner ?? '')
  if (!item.owner) return null
  return (
    <Group gap={4} mt={4}>
      <Badge size="sm" variant="light" color="grape" leftSection={<IconPuzzle size={12} />}>
        {t('admin.data.owner', { name })}
      </Badge>
      {item.ownerEnabled === false && (
        <Badge size="sm" variant="outline" color="gray">
          {t('admin.data.ownerOff')}
        </Badge>
      )}
    </Group>
  )
}

/** The data space: values and tables the site and, later, modules use. */
export function DataPage() {
  const { t, i18n } = useTranslation()
  const items = useDataItems()
  const [creating, setCreating] = useState(false)
  const [filter, setFilter] = useState<Visibility | 'all'>('all')
  const shown = items.data?.filter((it) => filter === 'all' || it.visibility === filter) ?? []
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
        <>
        <SegmentedControl aria-label={t('admin.data.filterLabel')} size="xs" style={{ alignSelf: 'flex-start' }} value={filter}
          onChange={(v) => setFilter(v as Visibility | 'all')}
          data={[{ value: 'all', label: t('admin.data.filterAll') }, ...visibilities.map((v) => ({ value: v, label: t(`admin.data.visibility.${v}`) }))]} />
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
            {shown.map((it) => (
              <Table.Tr key={it.key}>
                <Table.Td>
                  <Anchor component={Link} to={`/admin/data/${it.key}`} ff="monospace">{it.key}</Anchor>
                  {it.description && <Text size="xs" c="dimmed">{it.description}</Text>}
                  <OwnerBadges item={it} />
                </Table.Td>
                <Table.Td>
                  {t(`admin.data.kind.${it.kind}`)}
                  {it.valueType && <Text size="xs" c="dimmed">{t(`admin.data.type.${it.valueType}`)}</Text>}
                </Table.Td>
                <Table.Td>
                  <VisibilityBadge value={it.visibility} />
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
        </>
      )}
      <DataCreateModal opened={creating} onClose={() => setCreating(false)} />
    </Stack>
  )
}
