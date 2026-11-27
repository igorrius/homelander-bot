import { Alert, Button, Card, Loader, Pagination, Stack } from '@mantine/core'
import { IconHistory } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { useAudit } from '../../api/admin'
import { errorText } from '../../lib/errors'
import { AuditTable } from './AuditTable'
import { PageHeader } from '../../components/PageHeader'

const pageSize = 25

export function AuditPage() {
  const { t } = useTranslation()
  const [params, setParams] = useSearchParams()
  const userId = Number(params.get('userId')) || undefined
  const page = Number(params.get('page')) || 1
  const audit = useAudit({ userId, page })
  const total = audit.data?.total ?? 0
  const setPage = (p: number) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('page', String(p))
      return next
    })
  return (
    <Stack>
      <PageHeader icon={IconHistory} title={t('admin.audit.title')}
        actions={userId && (
          <Button variant="light" size="xs" onClick={() => setParams({})}>
            {t('admin.audit.clearFilter')}
          </Button>
        )} />
      {audit.isPending ? <Loader /> : audit.isError ? <Alert color="red">{errorText(t, audit.error)}</Alert> : <Card p={0}><AuditTable entries={audit.data.items} /></Card>}
      {total > pageSize && <Pagination total={Math.ceil(total / pageSize)} value={page} onChange={setPage} />}
    </Stack>
  )
}
