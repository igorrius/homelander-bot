import { ActionIcon, Anchor, Button, CopyButton, Group, Paper, Stack, Text, Tooltip } from '@mantine/core'
import { IconCheck, IconCopy, IconDownload } from '@tabler/icons-react'
import { QRCodeSVG } from 'qrcode.react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

const downloadSize = 1024

/** Saves the QR code as a large PNG: the SVG is drawn onto a canvas once the browser has loaded it. */
function downloadPng(svg: SVGSVGElement | null) {
  if (!svg) return
  const src = URL.createObjectURL(new Blob([new XMLSerializer().serializeToString(svg)], { type: 'image/svg+xml' }))
  const img = new Image()
  img.onload = () => {
    try {
      const canvas = document.createElement('canvas')
      canvas.width = canvas.height = downloadSize
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      ctx.imageSmoothingEnabled = false
      ctx.drawImage(img, 0, 0, downloadSize, downloadSize)
      const a = document.createElement('a')
      a.href = canvas.toDataURL('image/png')
      a.download = 'registration-qr.png'
      a.click()
    } finally {
      URL.revokeObjectURL(src)
    }
  }
  img.onerror = () => URL.revokeObjectURL(src)
  img.src = src
}

/** The public registration link as a QR code and text, with copy and "download QR" (PNG for printing or chats). */
export function InviteLink({ url }: { url: string }) {
  const { t } = useTranslation()
  const svg = useRef<SVGSVGElement>(null)
  return (
    <Group align="center" gap="lg" wrap="wrap">
      <Paper withBorder p={6} bg="white" style={{ lineHeight: 0 }}>
        <QRCodeSVG ref={svg} value={url} size={168} title={t('qr.title')} marginSize={1} />
      </Paper>
      <Stack gap="sm" style={{ flex: '1 1 260px', minWidth: 0 }}>
        <Text size="sm" c="dimmed">
          {t('admin.invite.hint')}
        </Text>
        <Group gap="xs" wrap="nowrap">
          <Anchor href={url} target="_blank" rel="noopener noreferrer" fw={500} style={{ wordBreak: 'break-all' }}>
            {url}
          </Anchor>
          <CopyButton value={url}>
            {({ copied, copy }) => (
              <Tooltip label={copied ? t('common.copied') : t('common.copy')} withArrow>
                <ActionIcon variant="light" onClick={copy} aria-label={t('common.copy')} color={copied ? 'green' : undefined}>
                  {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
                </ActionIcon>
              </Tooltip>
            )}
          </CopyButton>
        </Group>
        <Group>
          <Button variant="default" size="xs" leftSection={<IconDownload size={16} />} onClick={() => downloadPng(svg.current)}>
            {t('admin.invite.download')}
          </Button>
        </Group>
      </Stack>
    </Group>
  )
}
