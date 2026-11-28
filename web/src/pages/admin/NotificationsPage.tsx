import { Alert, Button, Card, Group, Loader, Select, Stack, Text } from '@mantine/core'
import { IconBell } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useChats, useLabels, useNotifications, useSaveNotifications } from '../../api/chats'
import { PageHeader } from '../../components/PageHeader'
import { reachesChat } from '../../lib/chats'
import { errorText, useApiErrors } from '../../lib/errors'

/** Where the system's own notices go. */
export function NotificationsPage() {
  const { t } = useTranslation()
  const settings = useNotifications()
  const labels = useLabels()
  const chats = useChats()
  const save = useSaveNotifications()
  // The admin's unsaved choice; undefined shows the saved one.
  const [draft, setLabelId] = useState<string | null | undefined>(undefined)
  const saved = settings.data?.applicationsLabelId
  const labelId = draft !== undefined ? draft : saved === undefined || saved === null ? null : String(saved)
  const { message, fields } = useApiErrors(save.error)
  const chosen = labels.data?.find((l) => String(l.id) === labelId)
  return (
    <Stack>
      <PageHeader icon={IconBell} title={t('admin.notifications.title')} description={t('admin.notifications.description')} />
      {settings.isPending || labels.isPending ? (
        <Loader />
      ) : settings.isError ? (
        <Alert color="red">{errorText(t, settings.error)}</Alert>
      ) : (
        <Card withBorder padding="md" maw={560}>
          <Stack>
            <Group align="flex-end" wrap="nowrap">
              <Select style={{ flex: 1 }} label={t('admin.notifications.applications')} description={t('admin.notifications.applicationsHint')}
                placeholder={t('admin.notifications.offPlaceholder')} data={(labels.data ?? []).map((l) => ({ value: String(l.id), label: l.name }))}
                value={labelId} onChange={setLabelId} error={fields.applicationsLabelId} />
              <Button variant="default" disabled={labelId === null} onClick={() => setLabelId(null)}>
                {t('admin.notifications.off')}
              </Button>
            </Group>
            {chosen && chats.data && !reachesChat(chosen, chats.data) && <Alert color="orange" p="xs">{t('admin.notifications.noChat')}</Alert>}
            {message && <Alert color="red">{message}</Alert>}
            {save.isSuccess && <Text size="sm" c="teal">{t('common.saved')}</Text>}
            <Group justify="flex-end">
              <Button loading={save.isPending} onClick={() => save.mutate({ applicationsLabelId: labelId === null ? null : Number(labelId) })}>
                {t('common.save')}
              </Button>
            </Group>
          </Stack>
        </Card>
      )}
    </Stack>
  )
}
