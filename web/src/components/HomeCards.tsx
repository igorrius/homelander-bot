import { Anchor, Box, Card, Divider, Group, Image, SimpleGrid, Stack, Text, ThemeIcon } from '@mantine/core'
import { IconExternalLink, IconInfoCircle, IconLink, IconList, IconPhone, IconPhoto } from '@tabler/icons-react'
import { Fragment, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { PublicItem } from '../api/publicData'
import type { DataColumn, DataType, HomeItem, LocalizedText } from '../api/types'
import { prettyKey } from '../lib/format'
import { useLocalized } from '../lib/localized'

const telHref = (phone: string) => `tel:${phone.replace(/[^\d+]/g, '')}`

function hostOf(url: string): string {
  try {
    const u = new URL(url)
    return (u.host + (u.pathname === '/' ? '' : u.pathname)).replace(/\/$/, '')
  } catch {
    return url
  }
}

/** One cell or value as text, a link or a picture; null and empty values show nothing. */
function Cell({ type, value, size = 'lg' }: { type: DataType; value: unknown; size?: 'sm' | 'lg' }) {
  const { t } = useTranslation()
  const text = useLocalized()
  if (value === null || value === undefined || value === '') return null
  switch (type) {
    case 'phone':
      return (
        <Anchor href={telHref(String(value))} size={size} fw={600} style={{ whiteSpace: 'nowrap' }}>
          {String(value)}
        </Anchor>
      )
    case 'url':
      return (
        <Anchor href={String(value)} target="_blank" rel="noopener noreferrer" size={size} fw={600} style={{ wordBreak: 'break-word' }}>
          {hostOf(String(value))} <IconExternalLink size={13} style={{ verticalAlign: '-1px' }} />
        </Anchor>
      )
    case 'image':
      return <Image src={String(value)} alt="" radius="sm" mah={size === 'sm' ? 64 : 220} w={size === 'sm' ? 64 : '100%'} fit="cover" />
    case 'bool':
      return <Text size={size}>{value ? t('common.yes') : t('common.no')}</Text>
    case 'localized_text':
      return <Text size={size} style={{ whiteSpace: 'pre-line' }}>{text(value as LocalizedText)}</Text>
    case 'json':
      return null
    default:
      return <Text size={size} style={{ whiteSpace: 'pre-line' }}>{String(value)}</Text>
  }
}

function hasValue(v: unknown): boolean {
  return v !== null && v !== undefined && v !== ''
}

function TableRows({ columns, rows }: { columns: DataColumn[]; rows: Record<string, unknown>[] }) {
  const shown = columns.filter((c) => c.type !== 'json')
  return (
    <Stack gap={0}>
      {rows.map((row, i) => (
        <Fragment key={String(row.id ?? i)}>
          {i > 0 && <Divider my={8} />}
          <Group gap={6} wrap="wrap" align="center">
            {shown
              .filter((c) => hasValue(row[c.name]))
              .map((c, k) => (
                <Fragment key={c.name}>
                  {k > 0 && <Text span c="dimmed" size="sm">·</Text>}
                  <Cell type={c.type} value={row[c.name]} size="sm" />
                </Fragment>
              ))}
          </Group>
        </Fragment>
      ))}
    </Stack>
  )
}

function CardIcon({ item }: { item: PublicItem }) {
  const props = { size: 18, stroke: 1.7 }
  if (item.kind === 'table') return <IconList {...props} />
  if (item.type === 'phone') return <IconPhone {...props} />
  if (item.type === 'url') return <IconLink {...props} />
  if (item.type === 'image') return <IconPhoto {...props} />
  return <IconInfoCircle {...props} />
}

function HomeCard({ title, item }: { title: string; item: PublicItem }) {
  let body: ReactNode
  if (item.kind === 'table') body = <TableRows columns={item.columns} rows={item.rows} />
  else body = <Cell type={item.type} value={item.value} />
  return (
    <Card component="li" radius="lg" padding="lg" shadow="xs" style={{ listStyle: 'none' }}>
      <Group gap="sm" mb="sm" wrap="nowrap">
        <ThemeIcon variant="light" radius="md" size={34}>
          <CardIcon item={item} />
        </ThemeIcon>
        <Text fw={650} size="sm" tt="uppercase" c="dimmed" style={{ letterSpacing: '0.04em' }}>
          {title}
        </Text>
      </Group>
      <Box>{body}</Box>
    </Card>
  )
}

/** The public data items the admin chose for the built-in page, as cards in their order. */
export function HomeCards({ items, data }: { items: HomeItem[]; data: Record<string, PublicItem> }) {
  const { t } = useTranslation()
  const text = useLocalized()
  const shown = items.filter((it) => Object.hasOwn(data, it.key))
  if (shown.length === 0) return null
  return (
    <SimpleGrid component="ul" aria-label={t('home.cards')} cols={{ base: 1, sm: 2, md: 3 }} spacing="lg" p={0} m={0}>
      {shown.map((it) => (
        <HomeCard key={it.key} title={text(it.title) || prettyKey(it.key)} item={data[it.key]} />
      ))}
    </SimpleGrid>
  )
}
