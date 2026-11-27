import { Alert, Badge, Button, Card, Group, Loader, Modal, NumberInput, Stack, Switch, Table, TextInput } from '@mantine/core'
import { useState, type FormEvent } from 'react'
import { IconPlus, IconTags } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useCreateStatus, useStatuses, useUpdateStatus, type StatusInput } from '../../api/admin'
import type { ResidencyStatus } from '../../api/types'
import { LocalizedTextInputs } from '../../components/LocalizedTextInputs'
import { errorText, useApiErrors } from '../../lib/errors'
import { cleanLocalized, useLocalized } from '../../lib/localized'
import { PageHeader } from '../../components/PageHeader'

function StatusForm({ initial, onDone }: { initial?: ResidencyStatus; onDone(): void }) {
  const { t } = useTranslation()
  const create = useCreateStatus()
  const update = useUpdateStatus(initial?.id ?? 0)
  const m = initial ? update : create
  const [code, setCode] = useState(initial?.code ?? '')
  const [names, setNames] = useState(initial?.names ?? {})
  const [sortOrder, setSortOrder] = useState<number | string>(initial?.sortOrder ?? 0)
  const [archived, setArchived] = useState(initial?.archived ?? false)
  const { message, fields } = useApiErrors(m.error)
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    const body: StatusInput = { code: code.trim(), names: cleanLocalized(names), sortOrder: Number(sortOrder) || 0, archived }
    m.mutate(body, { onSuccess: onDone })
  }
  return (
    <form onSubmit={onSubmit}>
      <Stack>
        <TextInput label={t('admin.statuses.code')} description={t('admin.statuses.codeHint')} value={code} disabled={!!initial}
          onChange={(e) => setCode(e.currentTarget.value)} error={fields.code} />
        <LocalizedTextInputs label={t('admin.statuses.name')} value={names} onChange={setNames} error={fields.names} />
        <NumberInput label={t('admin.statuses.sortOrder')} value={sortOrder} onChange={setSortOrder} allowDecimal={false} />
        {initial && <Switch label={t('admin.statuses.archived')} checked={archived} onChange={(e) => setArchived(e.currentTarget.checked)} />}
        {message && <Alert color="red">{message}</Alert>}
        <Group justify="flex-end">
          <Button type="submit" loading={m.isPending}>
            {t('common.save')}
          </Button>
        </Group>
      </Stack>
    </form>
  )
}

export function StatusesPage() {
  const { t } = useTranslation()
  const text = useLocalized()
  const statuses = useStatuses()
  const [editing, setEditing] = useState<ResidencyStatus | 'new' | null>(null)
  return (
    <Stack>
      <PageHeader icon={IconTags} title={t('admin.statuses.title')}
        actions={<Button leftSection={<IconPlus size={16} />} onClick={() => setEditing('new')}>{t('admin.statuses.add')}</Button>} />
      {statuses.isPending ? (
        <Loader />
      ) : statuses.isError ? (
        <Alert color="red">{errorText(t, statuses.error)}</Alert>
      ) : (
        <Card p={0}>
        <Table striped>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('admin.statuses.name')}</Table.Th>
              <Table.Th>{t('admin.statuses.sortOrder')}</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {statuses.data.map((s) => (
              <Table.Tr key={s.id}>
                <Table.Td>
                  {text(s.names)} {s.archived && <Badge color="gray">{t('admin.statuses.archived')}</Badge>}
                </Table.Td>
                <Table.Td>{s.sortOrder}</Table.Td>
                <Table.Td>
                  <Button size="xs" variant="subtle" onClick={() => setEditing(s)}>
                    {t('common.edit')}
                  </Button>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
        </Card>
      )}
      <Modal opened={editing !== null} onClose={() => setEditing(null)} title={editing === 'new' ? t('admin.statuses.add') : t('common.edit')} centered>
        {editing !== null && <StatusForm initial={editing === 'new' ? undefined : editing} onDone={() => setEditing(null)} />}
      </Modal>
    </Stack>
  )
}
