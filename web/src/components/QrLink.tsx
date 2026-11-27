import { ActionIcon, Anchor, CopyButton, Group, Stack, Tooltip } from '@mantine/core'
import { IconCheck, IconCopy } from '@tabler/icons-react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'

/** A link as a QR code (for phones) and as text, optionally with a copy button. */
export function QrLink({ url, copy = false }: { url: string; copy?: boolean }) {
  const { t } = useTranslation()
  return (
    <Stack gap="xs" align="flex-start">
      <QRCodeSVG value={url} size={160} title={t('qr.title')} marginSize={2} />
      <Group gap="xs">
        <Anchor href={url} target="_blank" rel="noopener noreferrer" style={{ wordBreak: 'break-all' }}>
          {url}
        </Anchor>
        {copy && (
          <CopyButton value={url}>
            {({ copied, copy: doCopy }) => (
              <Tooltip label={copied ? t('common.copied') : t('common.copy')} withArrow>
                <ActionIcon variant="light" onClick={doCopy} aria-label={t('common.copy')}>
                  {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
                </ActionIcon>
              </Tooltip>
            )}
          </CopyButton>
        )}
      </Group>
    </Stack>
  )
}
