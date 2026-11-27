import { Alert, Button, Card, Loader, Modal, Stack } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { IconInbox, IconQrcode } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useUsers } from '../../api/admin'
import { useSite } from '../../api/public'
import { EmptyState } from '../../components/EmptyState'
import { InviteLink } from '../../components/InviteLink'
import { PageHeader } from '../../components/PageHeader'
import { errorText } from '../../lib/errors'
import { ApplicationList } from './ApplicationList'

export function ApplicationsPage() {
  const { t } = useTranslation()
  const users = useUsers({ status: 'pending_approval', pageSize: 100 })
  const site = useSite()
  const [inviting, { open, close }] = useDisclosure(false)
  return (
    <Stack>
      <PageHeader icon={IconInbox} title={t('admin.applications.title')} description={t('admin.applications.hint')}
        actions={
          <Button variant="light" leftSection={<IconQrcode size={16} />} onClick={open}>
            {t('admin.invite.open')}
          </Button>
        } />
      {users.isPending ? (
        <Loader />
      ) : users.isError ? (
        <Alert color="red">{errorText(t, users.error)}</Alert>
      ) : users.data.items.length === 0 ? (
        <Card>
          <EmptyState icon={IconInbox} title={t('admin.applications.empty')} hint={t('admin.applications.emptyHint')}
            action={<Button variant="light" leftSection={<IconQrcode size={16} />} onClick={open}>{t('admin.invite.open')}</Button>} />
        </Card>
      ) : (
        <ApplicationList users={users.data.items} />
      )}
      <Modal opened={inviting} onClose={close} title={t('admin.invite.title')} size="lg" centered>
        {site.data ? <InviteLink url={site.data.registerUrl} /> : site.isError ? <Alert color="red">{errorText(t, site.error)}</Alert> : <Loader size="sm" />}
      </Modal>
    </Stack>
  )
}
