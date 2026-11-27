import { ActionIcon, Alert, Badge, Box, Button, Card, Grid, Group, Loader, Paper, Select, Stack, Text, Title } from '@mantine/core'
import { useElementSize } from '@mantine/hooks'
import { notifications } from '@mantine/notifications'
import { IconArrowDown, IconArrowUp, IconEye, IconLayoutCards, IconTextCaption, IconX } from '@tabler/icons-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useHouses, useSaveSite } from '../../api/admin'
import { useDataItems } from '../../api/data'
import { usePublicData } from '../../api/publicData'
import type { DataItem, HomeItem, Locale, Site } from '../../api/types'
import { BuiltInHome } from '../../components/BuiltInHome'
import { LocalizedTextInputs } from '../../components/LocalizedTextInputs'
import { SiteHeader } from '../../components/SiteHeader'
import { supportedLocales } from '../../i18n'
import { useApiErrors } from '../../lib/errors'
import { cleanLocalized, localized } from '../../lib/localized'

/** Items that can be a card: public, and not raw JSON. */
function cardable(it: DataItem): boolean {
  return it.public && !(it.kind === 'value' && it.valueType === 'json')
}

const PREVIEW_WIDTH = 1180

/** The built-in page at desktop width, scaled down to the box; not clickable. */
function MiniPreview({ name, welcome, items, defaultLocale }: { name: Site['communityName']; welcome: Site['welcomeText']; items: HomeItem[]; defaultLocale: Locale }) {
  const { t, i18n } = useTranslation()
  const houses = useHouses()
  const data = usePublicData(items.length > 0)
  const { ref, width } = useElementSize()
  const inner = useElementSize()
  const scale = width > 0 ? width / PREVIEW_WIDTH : 0.35
  const title = localized(name, i18n.language, defaultLocale) || t('home.title')
  const text = localized(welcome, i18n.language, defaultLocale) || t('home.welcomeDefault')
  const list = houses.data?.filter((h) => !h.archived).sort((a, b) => a.sortOrder - b.sortOrder)
  return (
    <Paper withBorder radius="md" style={{ overflow: 'hidden' }} ref={ref} data-testid="home-preview">
      <Box style={{ height: inner.height ? inner.height * scale : 360, overflow: 'hidden' }}>
        <Box ref={inner.ref} inert w={PREVIEW_WIDTH} style={{ transform: `scale(${scale})`, transformOrigin: 'top left', background: 'var(--mantine-color-body)' }}>
          <SiteHeader title={title} />
          <BuiltInHome name={title} welcome={text} houses={list} homeItems={items} data={data.data} />
        </Box>
      </Box>
    </Paper>
  )
}

function ItemRow({ item, index, count, known, onChange, onMove, onRemove }: {
  item: HomeItem
  index: number
  count: number
  known: DataItem | undefined
  onChange(it: HomeItem): void
  onMove(to: number): void
  onRemove(): void
}) {
  const { t } = useTranslation()
  return (
    <Paper withBorder p="sm" radius="md" data-key={item.key}>
      <Group justify="space-between" mb={6} wrap="nowrap">
        <Group gap="xs" wrap="nowrap" style={{ minWidth: 0 }}>
          <Badge variant="light" circle>{index + 1}</Badge>
          <Text ff="monospace" fw={600} size="sm" truncate>{item.key}</Text>
          {known ? (
            <Badge size="xs" variant="default" tt="none">{known.kind === 'table' ? t('admin.data.kind.table') : t(`admin.data.type.${known.valueType}`)}</Badge>
          ) : (
            <Badge size="xs" color="orange" variant="light" tt="none">{t('admin.site.notPublic')}</Badge>
          )}
        </Group>
        <Group gap={4} wrap="nowrap">
          <ActionIcon variant="subtle" color="gray" aria-label={t('admin.site.moveUp')} disabled={index === 0} onClick={() => onMove(index - 1)}>
            <IconArrowUp size={16} />
          </ActionIcon>
          <ActionIcon variant="subtle" color="gray" aria-label={t('admin.site.moveDown')} disabled={index === count - 1} onClick={() => onMove(index + 1)}>
            <IconArrowDown size={16} />
          </ActionIcon>
          <ActionIcon variant="subtle" color="red" aria-label={t('admin.site.removeItem')} onClick={onRemove}>
            <IconX size={16} />
          </ActionIcon>
        </Group>
      </Group>
      <LocalizedTextInputs label={t('admin.site.cardTitle')} value={item.title} onChange={(title) => onChange({ ...item, title })} />
    </Paper>
  )
}

/** The Built-in page tab: texts, default language, the cards under the quarter, and a live preview. */
export function SiteHomeTab({ site }: { site: Site }) {
  const { t } = useTranslation()
  const save = useSaveSite()
  const data = useDataItems()
  const [communityName, setName] = useState(site.communityName)
  const [welcomeText, setWelcome] = useState(site.welcomeText)
  const [defaultLocale, setDefaultLocale] = useState<Locale>(site.defaultLocale)
  const [items, setItems] = useState<HomeItem[]>(site.homeItems ?? [])
  const { message, fields } = useApiErrors(save.error)
  const options = (data.data ?? []).filter(cardable)
  const byKey = new Map(options.map((it) => [it.key, it]))
  const free = options.filter((it) => !items.some((x) => x.key === it.key))

  const move = (from: number, to: number) => {
    const next = [...items]
    const [it] = next.splice(from, 1)
    next.splice(to, 0, it)
    setItems(next)
  }
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    save.mutate(
      {
        communityName: cleanLocalized(communityName), welcomeText: cleanLocalized(welcomeText), defaultLocale,
        homeItems: items.map((it) => ({ key: it.key, title: cleanLocalized(it.title) })),
      },
      { onSuccess: () => notifications.show({ message: t('common.saved'), color: 'green' }) },
    )
  }

  return (
    <Grid gap="xl">
      <Grid.Col span={{ base: 12, lg: 6 }}>
        <form onSubmit={onSubmit}>
          <Stack gap="lg">
            <Card withBorder radius="md">
              <Stack>
                <Group gap="xs"><IconTextCaption size={18} /><Title order={3} size="h5">{t('admin.site.texts')}</Title></Group>
                <LocalizedTextInputs label={t('admin.site.communityName')} value={communityName} onChange={setName} error={fields.communityName} />
                <LocalizedTextInputs label={t('admin.site.welcomeText')} value={welcomeText} onChange={setWelcome} error={fields.welcomeText} multiline />
                <Select label={t('admin.site.defaultLocale')} value={defaultLocale} onChange={(v) => v && setDefaultLocale(v as Locale)} allowDeselect={false}
                  data={supportedLocales.map((l) => ({ value: l, label: t(`language.${l}`) }))} error={fields.defaultLocale} w={220} />
              </Stack>
            </Card>
            <Card withBorder radius="md">
              <Stack>
                <div>
                  <Group gap="xs"><IconLayoutCards size={18} /><Title order={3} size="h5">{t('admin.site.cardsTitle')}</Title></Group>
                  <Text size="sm" c="dimmed">{t('admin.site.cardsHint')}</Text>
                </div>
                {items.length === 0 && <Text size="sm" c="dimmed">{t('admin.site.noCards')}</Text>}
                {items.map((it, i) => (
                  <ItemRow key={it.key} item={it} index={i} count={items.length} known={byKey.get(it.key)}
                    onChange={(next) => setItems(items.map((x, k) => (k === i ? next : x)))}
                    onMove={(to) => move(i, to)} onRemove={() => setItems(items.filter((_, k) => k !== i))} />
                ))}
                {data.isPending ? (
                  <Loader size="sm" />
                ) : options.length === 0 ? (
                  <Alert color="gray" variant="light">{t('admin.site.noPublic')}</Alert>
                ) : (
                  free.length > 0 && (
                    <Select label={t('admin.site.addItem')} placeholder={t('admin.site.addItem')} value={null} searchable
                      data={free.map((it) => ({ value: it.key, label: it.key }))} error={fields.homeItems}
                      onChange={(key) => key && setItems([...items, { key, title: {} }])} />
                  )
                )}
              </Stack>
            </Card>
            {message && <Alert color="red">{message}</Alert>}
            <Group>
              <Button type="submit" loading={save.isPending}>{t('admin.site.save')}</Button>
            </Group>
          </Stack>
        </form>
      </Grid.Col>
      <Grid.Col span={{ base: 12, lg: 6 }}>
        <Box style={{ position: 'sticky', top: 76 }}>
          <Group gap="xs" mb={4}><IconEye size={18} /><Title order={3} size="h5">{t('admin.site.preview')}</Title></Group>
          <Text size="xs" c="dimmed" mb="sm">{t('admin.site.previewHint')}</Text>
          <MiniPreview name={communityName} welcome={welcomeText} items={items} defaultLocale={defaultLocale} />
        </Box>
      </Grid.Col>
    </Grid>
  )
}
