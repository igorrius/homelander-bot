import { Anchor, Code, Image, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { adminAssetUrl } from '../../api/data'
import type { DataType, LocalizedText } from '../../api/types'

const empty = (v: unknown) => v === undefined || v === null || v === ''

/** A value or table cell as text, for items that cannot be edited here. */
export function DataValueView({ type, value, compact }: { type: DataType | undefined; value: unknown; compact?: boolean }) {
  const { t } = useTranslation()
  const size = compact ? 'sm' : 'md'
  if (type === 'image') {
    if (typeof value !== 'number') return compact ? null : <Text c="dimmed">{t('admin.data.noImage')}</Text>
    return <Image src={adminAssetUrl(value)} alt="" h={compact ? 40 : 240} w="auto" fit="contain" radius="sm" />
  }
  if (empty(value)) return compact ? null : <Text c="dimmed">—</Text>
  switch (type) {
    case 'bool':
      return <Text size={size}>{value === true ? t('common.yes') : t('common.no')}</Text>
    case 'url':
      return (
        <Anchor href={String(value)} target="_blank" rel="noopener noreferrer" size={size} style={{ overflowWrap: 'anywhere' }}>
          {String(value)}
        </Anchor>
      )
    case 'json':
      return <Code block>{JSON.stringify(value, null, 2)}</Code>
    case 'localized_text': {
      const entries = Object.entries((value ?? {}) as LocalizedText).filter(([, v]) => v)
      return (
        <Stack gap={2}>
          {entries.map(([loc, v]) => (
            <Text key={loc} size={size} style={{ whiteSpace: 'pre-wrap' }}>
              <Text span c="dimmed" size="xs" mr={4}>
                {loc}
              </Text>
              {v}
            </Text>
          ))}
        </Stack>
      )
    }
    default:
      return (
        <Text size={size} style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }} lineClamp={compact ? 4 : undefined}>
          {String(value)}
        </Text>
      )
  }
}
