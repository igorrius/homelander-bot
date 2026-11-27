import { ActionIcon, Alert, Button, Card, Checkbox, FileButton, Group, Image, Select, Stack, Table, Text, TextInput, Title } from '@mantine/core'
import { IconArrowDown, IconArrowUp, IconPlus, IconTrash } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { adminAssetUrl, useDeleteDataRow, usePatchDataItem, usePutDataRows, useUploadDataImage } from '../../api/data'
import { columnTypes, type DataColumn, type DataItem, type DataRow, type DataType, type LocalizedText } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { supportedLocales } from '../../i18n'
import { useApiErrors } from '../../lib/errors'

type DraftColumn = DataColumn & { isNew?: boolean }

/** A cell is sent only when it holds something; an omitted cell is empty. */
function isEmpty(v: unknown): boolean {
  if (v === undefined || v === null || v === '') return true
  if (typeof v === 'object' && !Array.isArray(v)) return Object.values(v as object).every((x) => x === undefined || x === '')
  return false
}

function Cell({ type, label, value, onChange }: { type: DataType; label: string; value: unknown; onChange(v: unknown): void }) {
  const upload = useUploadDataImage()
  switch (type) {
    case 'bool':
      return <Checkbox aria-label={label} checked={value === true} onChange={(e) => onChange(e.currentTarget.checked)} />
    case 'number':
      return <TextInput aria-label={label} type="number" size="xs" value={value === undefined || value === null ? '' : String(value)}
        onChange={(e) => onChange(e.currentTarget.value === '' ? null : Number(e.currentTarget.value))} />
    case 'localized_text': {
      const v = (value ?? {}) as LocalizedText
      return (
        <Stack gap={2}>
          {supportedLocales.map((loc) => (
            <TextInput key={loc} aria-label={`${label} (${loc})`} size="xs" placeholder={loc} value={v[loc] ?? ''}
              onChange={(e) => onChange({ ...v, [loc]: e.currentTarget.value })} />
          ))}
        </Stack>
      )
    }
    case 'image':
      return (
        <Group gap={4} wrap="nowrap">
          {typeof value === 'number' && <Image src={adminAssetUrl(value)} alt="" h={32} w={32} fit="contain" />}
          <FileButton accept="image/png,image/jpeg,image/webp,image/gif" onChange={(f) => f && upload.mutate(f, { onSuccess: (a) => onChange(a.id) })}>
            {(props) => <Button {...props} size="compact-xs" variant="default" aria-label={label} loading={upload.isPending}>…</Button>}
          </FileButton>
        </Group>
      )
    default:
      return <TextInput aria-label={label} size="xs" value={typeof value === 'string' ? value : ''} onChange={(e) => onChange(e.currentTarget.value)} />
  }
}

/** Columns and rows of a table item; rows are saved together in one request. */
export function DataTableEditor({ item, rows }: { item: DataItem; rows: DataRow[] }) {
  const { t } = useTranslation()
  const patch = usePatchDataItem(item.key)
  const put = usePutDataRows(item.key)
  const delRow = useDeleteDataRow(item.key)
  const saved = item.columns ?? []
  const [cols, setCols] = useState<DraftColumn[]>(saved)
  const [draft, setDraft] = useState<DataRow[]>(rows)
  const colErr = useApiErrors(patch.error)
  const rowErr = useApiErrors(put.error ?? delRow.error)
  const rowFieldErrors = Object.entries(rowErr.fields)

  const setCol = (i: number, c: Partial<DraftColumn>) => setCols(cols.map((x, j) => (j === i ? { ...x, ...c } : x)))
  const setCell = (i: number, name: string, v: unknown) =>
    setDraft(draft.map((r, j) => (j === i ? { ...r, cells: { ...r.cells, [name]: v } } : r)))
  const move = (i: number, d: number) => {
    const next = [...draft]
    ;[next[i], next[i + d]] = [next[i + d], next[i]]
    setDraft(next)
  }
  const saveColumns = () =>
    patch.mutate({ columns: cols.map(({ name, label, type }) => ({ name: name.trim(), label: label.trim() || name.trim(), type })) })
  const saveRows = () =>
    put.mutate(
      draft.map((r, i) => ({
        ...(r.id ? { id: r.id } : {}),
        position: i,
        cells: Object.fromEntries(Object.entries(r.cells).filter(([k, v]) => saved.some((c) => c.name === k) && !isEmpty(v))),
      })),
    )

  return (
    <Stack>
      <Card withBorder>
        <Stack>
          <Title order={4}>{t('admin.data.columns')}</Title>
          <Table>
            <Table.Tbody>
              {cols.map((c, i) => (
                <Table.Tr key={i}>
                  <Table.Td>
                    <TextInput aria-label={t('admin.data.columnName')} placeholder={t('admin.data.columnName')} ff="monospace" value={c.name}
                      disabled={!c.isNew} onChange={(e) => setCol(i, { name: e.currentTarget.value })} />
                  </Table.Td>
                  <Table.Td>
                    <TextInput aria-label={t('admin.data.columnLabel')} placeholder={t('admin.data.columnLabel')} value={c.label}
                      onChange={(e) => setCol(i, { label: e.currentTarget.value })} />
                  </Table.Td>
                  <Table.Td>
                    <Select aria-label={t('admin.data.columnType')} value={c.type} allowDeselect={false} disabled={!c.isNew && item.rowCount > 0}
                      data={columnTypes.map((v) => ({ value: v, label: t(`admin.data.type.${v}`) }))} onChange={(v) => v && setCol(i, { type: v as DataType })} />
                  </Table.Td>
                  <Table.Td>
                    {c.isNew ? (
                      <ActionIcon variant="subtle" color="red" aria-label={`${t('admin.data.removeColumn')} ${c.name}`} onClick={() => setCols(cols.filter((_, j) => j !== i))}>
                        <IconTrash size={16} />
                      </ActionIcon>
                    ) : (
                      <ConfirmButton size="compact-sm" variant="subtle" color="red" title={t('admin.data.removeColumn')} aria-label={`${t('admin.data.removeColumn')} ${c.name}`}
                        message={t('admin.data.removeColumnConfirm', { name: c.name })} onConfirm={() => setCols(cols.filter((_, j) => j !== i))}>
                        <IconTrash size={16} />
                      </ConfirmButton>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
          {colErr.fields.columns && <Alert color="red">{colErr.fields.columns}</Alert>}
          {colErr.message && <Alert color="red">{colErr.message}</Alert>}
          <Group justify="space-between">
            <Button variant="default" leftSection={<IconPlus size={16} />} onClick={() => setCols([...cols, { name: '', label: '', type: 'text', isNew: true }])}>
              {t('admin.data.addColumn')}
            </Button>
            <Button onClick={saveColumns} loading={patch.isPending}>{t('admin.data.saveColumns')}</Button>
          </Group>
        </Stack>
      </Card>

      <Card withBorder>
        <Stack>
          <Title order={4}>{t('admin.data.rowsTitle')}</Title>
          {saved.length === 0 ? (
            <Text c="dimmed">{t('admin.data.noColumns')}</Text>
          ) : (
            <Table.ScrollContainer minWidth={400}>
              <Table>
                <Table.Thead>
                  <Table.Tr>
                    {saved.map((c) => (
                      <Table.Th key={c.name}>{c.label || c.name}</Table.Th>
                    ))}
                    <Table.Th />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {draft.map((r, i) => (
                    <Table.Tr key={r.id ?? `new-${i}`}>
                      {saved.map((c) => (
                        <Table.Td key={c.name}>
                          <Cell type={c.type} label={`${c.label || c.name} ${i + 1}`} value={r.cells[c.name]} onChange={(v) => setCell(i, c.name, v)} />
                        </Table.Td>
                      ))}
                      <Table.Td>
                        <Group gap={2} wrap="nowrap">
                          <ActionIcon variant="subtle" aria-label={t('admin.data.moveUp')} disabled={i === 0} onClick={() => move(i, -1)}>
                            <IconArrowUp size={16} />
                          </ActionIcon>
                          <ActionIcon variant="subtle" aria-label={t('admin.data.moveDown')} disabled={i === draft.length - 1} onClick={() => move(i, 1)}>
                            <IconArrowDown size={16} />
                          </ActionIcon>
                          {r.id ? (
                            <ConfirmButton size="compact-xs" variant="subtle" color="red" message={t('admin.data.deleteRowConfirm')}
                              onConfirm={() => r.id && delRow.mutate(r.id)}>
                              {t('common.delete')}
                            </ConfirmButton>
                          ) : (
                            <Button size="compact-xs" variant="subtle" color="red" onClick={() => setDraft(draft.filter((_, j) => j !== i))}>
                              {t('common.delete')}
                            </Button>
                          )}
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          )}
          {draft.length === 0 && saved.length > 0 && <Text c="dimmed">{t('admin.data.noRows')}</Text>}
          {rowFieldErrors.length > 0 && <Alert color="red">{rowFieldErrors.map(([f, m]) => `${f}: ${m}`).join('; ')}</Alert>}
          {rowErr.message && <Alert color="red">{rowErr.message}</Alert>}
          <Group justify="space-between">
            <Button variant="default" leftSection={<IconPlus size={16} />} disabled={saved.length === 0}
              onClick={() => setDraft([...draft, { position: draft.length, cells: {} }])}>
              {t('admin.data.addRow')}
            </Button>
            <Button onClick={saveRows} loading={put.isPending} disabled={saved.length === 0}>{t('admin.data.saveRows')}</Button>
          </Group>
        </Stack>
      </Card>
    </Stack>
  )
}
