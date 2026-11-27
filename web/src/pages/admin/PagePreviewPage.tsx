import { Alert, Box, Button, Card, Group, Loader, SegmentedControl, Select, Stack, Table, Text } from '@mantine/core'
import { IconLayout } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { useCopyPage, useDeletePage, usePagePreview, usePageVersion, usePublishPage } from '../../api/pages'
import { useSite } from '../../api/public'
import type { PageVersion } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageFrame } from '../../components/PageFrame'
import { PageHeader } from '../../components/PageHeader'
import { SiteHeader } from '../../components/SiteHeader'
import { errorText } from '../../lib/errors'
import { formatDateTime, formatSize } from '../../lib/format'
import { BackLink } from '../../components/BackLink'
import { useLocalized } from '../../lib/localized'
import { PageStatusBadge } from './PageStatusBadge'

function Actions({ version }: { version: PageVersion }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const publish = usePublishPage()
  const copy = useCopyPage()
  const del = useDeletePage()
  const error = publish.error ?? copy.error ?? del.error
  return (
    <Stack gap="xs" align="flex-end">
      <Group gap="sm">
        {version.status !== 'published' && (
          <ConfirmButton message={t('admin.pages.publishConfirm', { id: version.id })} loading={publish.isPending}
            onConfirm={() => publish.mutate(version.id)}>
            {t('admin.pages.publish')}
          </ConfirmButton>
        )}
        <Button variant="default" loading={copy.isPending}
          onClick={() => copy.mutate(version.id, { onSuccess: (v) => navigate(`/admin/pages/${v.id}`) })}>
          {t('admin.pages.copy')}
        </Button>
        {version.status !== 'published' && (
          <ConfirmButton color="red" variant="light" message={t('admin.pages.deleteConfirm', { id: version.id })}
            loading={del.isPending} onConfirm={() => del.mutate(version.id, { onSuccess: () => navigate('/admin/site?tab=pages') })}>
            {t('admin.pages.delete')}
          </ConfirmButton>
        )}
      </Group>
      {error && <Alert color="red">{errorText(t, error)}</Alert>}
    </Stack>
  )
}

/** Up to this many pages get a segmented control; more get a select. */
const SEGMENTED_MAX = 6

function PageSwitcher({ pages, value, onChange }: { pages: string[]; value: string; onChange(path: string): void }) {
  const { t } = useTranslation()
  if (pages.length <= SEGMENTED_MAX) {
    return <SegmentedControl aria-label={t('admin.pages.previewPage')} size="xs" value={value} onChange={onChange}
      data={pages.map((p) => ({ value: p, label: p }))} styles={{ label: { fontFamily: 'var(--mantine-font-family-monospace)' } }} />
  }
  return <Select aria-label={t('admin.pages.previewPage')} size="xs" w={220} value={value} allowDeselect={false} searchable
    onChange={(v) => v && onChange(v)} data={pages} styles={{ input: { fontFamily: 'var(--mantine-font-family-monospace)' } }} />
}

/** The site as visitors would see it with this version: our header, then the frame, with a page switcher. */
function Preview({ version }: { version: PageVersion }) {
  const { t, i18n } = useTranslation()
  const preview = usePagePreview(version.id)
  const site = useSite()
  const text = useLocalized()
  const pages = version.pages.length ? version.pages : [{ name: 'index', path: '/' }]
  const paths = pages.map((p) => p.path)
  const [current, setCurrent] = useState('/')
  const page = pages.find((p) => p.path === current) ?? pages[0]
  if (preview.isPending) return <Loader />
  if (preview.isError) return <Alert color="red">{errorText(t, preview.error)}</Alert>
  // The preview link names index.html; the other pages sit next to it.
  const base = preview.data.url.slice(0, preview.data.url.lastIndexOf('/') + 1)
  const name = text(site.data?.communityName) || t('home.title')
  return (
    <Card withBorder padding={0} style={{ overflow: 'hidden' }}>
      <Group justify="space-between" px="md" py="xs" gap="sm" bg="light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))"
        style={{ borderBottom: '1px solid var(--mantine-color-default-border)' }}>
        <PageSwitcher pages={paths} value={page.path} onChange={setCurrent} />
        <Text size="xs" c="dimmed">{t('admin.pages.previewHint')}</Text>
      </Group>
      {/* Our real header, inert here: the preview shows the layout, not a second account control. */}
      <Box inert>
        <SiteHeader title={name} />
      </Box>
      <PageFrame src={base + page.name + '.html'} lang={i18n.language} title={t('admin.pages.previewTitle')}
        pages={paths} onNavigate={setCurrent} />
    </Card>
  )
}

/** One page version: its files, publishing, and a live preview. Agents link admins here. */
export function PagePreviewPage() {
  const { t, i18n } = useTranslation()
  const id = Number(useParams().id)
  const q = usePageVersion(id)
  if (q.isPending) return <Loader />
  if (q.isError) return <Alert color="red">{errorText(t, q.error)}</Alert>
  const v = q.data
  return (
    <Stack>
      <BackLink to="/admin/site?tab=pages">{t('admin.pages.back')}</BackLink>
      <PageHeader icon={IconLayout} title={t('admin.pages.versionTitle', { id: v.id })}
        description={<Group gap="xs"><PageStatusBadge status={v.status} />{v.note && <Text size="sm">{v.note}</Text>}</Group>}
        actions={<Actions version={v} />} />
      <Text size="sm" c="dimmed">
        {t('admin.pages.createdBy', { at: formatDateTime(v.createdAt, i18n.language), name: v.createdBy?.name ?? '—' })}
      </Text>
      <Table withTableBorder>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('admin.pages.path')}</Table.Th>
            <Table.Th>{t('admin.pages.type')}</Table.Th>
            <Table.Th>{t('admin.pages.size')}</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {v.files.map((f) => (
            <Table.Tr key={f.path}>
              <Table.Td ff="monospace">{f.path}</Table.Td>
              <Table.Td>{f.contentType.split(';')[0]}</Table.Td>
              <Table.Td>{formatSize(f.size)}</Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Preview version={v} />
    </Stack>
  )
}
