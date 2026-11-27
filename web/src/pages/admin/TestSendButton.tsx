import { Button, Group, Text } from '@mantine/core'
import { IconSend } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useOutboxStatus, useTestSend, type TestTarget } from '../../api/chats'
import { errorText } from '../../lib/errors'

/** Sends a test message to a chat, topic or label and shows whether it arrived. */
export function TestSendButton({ target }: { target: TestTarget }) {
  const { t } = useTranslation()
  const send = useTestSend()
  const [id, setId] = useState<number | null>(null)
  const status = useOutboxStatus(id)
  let result: string | null = null
  let color = 'dimmed'
  if (send.isError) {
    result = errorText(t, send.error)
    color = 'red'
  } else if (status.data?.state === 'sent') {
    result = t('admin.chats.testSent')
    color = 'teal'
  } else if (status.data?.state === 'failed') {
    result = t('admin.chats.testFailed', { reason: status.data.lastError })
    color = 'red'
  } else if (id !== null) {
    result = t('admin.chats.testQueued')
  }
  return (
    <Group gap="xs" wrap="nowrap">
      <Button size="xs" variant="subtle" leftSection={<IconSend size={14} />} loading={send.isPending}
        onClick={() => send.mutate(target, { onSuccess: (r) => setId(r.outboxIds[0] ?? null) })}>
        {t('admin.chats.test')}
      </Button>
      {result && <Text size="xs" c={color}>{result}</Text>}
    </Group>
  )
}
