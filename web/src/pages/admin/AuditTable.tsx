import { Anchor, Table, Text } from '@mantine/core'
import { IconHistory } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { AuditEntry } from '../../api/types'
import { EmptyState } from '../../components/EmptyState'
import { useAuditAction } from '../../lib/audit'
import { formatDateTime } from '../../lib/format'
import { useLocalized } from '../../lib/localized'

/** Where an audit target links to, by its kind ("user:5" → that user's card). */
function targetHref(target: string): string | null {
  const m = /^(user|house|status|chat|topic|label):(\d+)$/.exec(target)
  if (!m) return null
  switch (m[1]) {
    case 'user':
      return `/admin/users/${m[2]}`
    case 'house':
      return `/admin/houses/${m[2]}`
    case 'status':
      return '/admin/statuses'
    case 'label':
      return '/admin/chats/labels'
    default:
      return '/admin/chats'
  }
}

/**
 * A target by name when the API sent one, linked to its page; otherwise the raw target.
 * People's names are not unique (residents choose their own), so their number stays visible.
 */
function Target({ target, name }: { target: string; name: string }) {
  const href = targetHref(target)
  if (!name || !href) return <>{target}</>
  const user = /^user:(\d+)$/.exec(target)
  return (
    <>
      <Anchor component={Link} to={href}>
        {name}
      </Anchor>
      {user && (
        <Text span size="xs" c="dimmed" ml={4}>
          #{user[1]}
        </Text>
      )}
    </>
  )
}

export function AuditTable({ entries }: { entries: AuditEntry[] }) {
  const { t, i18n } = useTranslation()
  const action = useAuditAction()
  const text = useLocalized()
  if (entries.length === 0) return <EmptyState icon={IconHistory} title={t('admin.audit.empty')} />
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
                <Target target={e.target} name={text(e.targetNames ?? {}) || e.targetName} />
              </Table.Td>
              <Table.Td>
                {e.actorUserId ? <Target target={`user:${e.actorUserId}`} name={e.actorName} /> : null}
                {e.details?.via === 'mcp' && (
                  <Text span size="xs" c="grape" ml={4}>
                    {` · ${t('admin.audit.viaMcp', { client: String(e.details.client ?? '') })}`}
                  </Text>
                )}
              </Table.Td>
              <Table.Td>{e.ip}</Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  )
}
