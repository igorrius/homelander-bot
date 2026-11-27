import { Anchor, Table, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { AuditEntry } from '../../api/types'
import { useAuditAction } from '../../lib/audit'
import { formatDateTime } from '../../lib/format'

/** "user:5" → link to the user card; other targets as text. */
function Target({ target }: { target: string }) {
  const m = /^user:(\d+)$/.exec(target)
  if (!m) return <>{target}</>
  return (
    <Anchor component={Link} to={`/admin/users/${m[1]}`}>
      {target}
    </Anchor>
  )
}

export function AuditTable({ entries }: { entries: AuditEntry[] }) {
  const { t, i18n } = useTranslation()
  const action = useAuditAction()
  if (entries.length === 0) return <Text c="dimmed">{t('admin.audit.empty')}</Text>
  return (
    <Table.ScrollContainer minWidth={640}>
      <Table striped>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('admin.audit.at')}</Table.Th>
            <Table.Th>{t('admin.audit.action')}</Table.Th>
            <Table.Th>{t('admin.audit.target')}</Table.Th>
            <Table.Th>{t('admin.audit.actor')}</Table.Th>
            <Table.Th>{t('admin.audit.ip')}</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {entries.map((e) => (
            <Table.Tr key={e.id}>
              <Table.Td>{formatDateTime(e.at, i18n.language)}</Table.Td>
              <Table.Td>{action(e.action)}</Table.Td>
              <Table.Td>
                <Target target={e.target} />
              </Table.Td>
              <Table.Td>{e.actorUserId ? <Target target={`user:${e.actorUserId}`} /> : '—'}</Table.Td>
              <Table.Td>{e.ip}</Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  )
}
