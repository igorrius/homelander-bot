import { Alert, Button, Group, Loader, Pagination, Stack, Title } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { useAudit } from '../../api/admin'
import { errorText } from '../../lib/errors'
import { AuditTable } from './AuditTable'

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
      <Group justify="space-between">
        <Title order={2}>{t('admin.audit.title')}</Title>
        {userId && (
          <Button variant="light" size="xs" onClick={() => setParams({})}>
            {t('admin.audit.clearFilter')}
          </Button>
        )}
      </Group>
      {audit.isPending ? <Loader /> : audit.isError ? <Alert color="red">{errorText(t, audit.error)}</Alert> : <AuditTable entries={audit.data.items} />}
      {total > pageSize && <Pagination total={Math.ceil(total / pageSize)} value={page} onChange={setPage} />}
    </Stack>
  )
}
