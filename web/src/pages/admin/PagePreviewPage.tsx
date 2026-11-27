import { Alert, Anchor, Button, Card, Group, Loader, Stack, Table, Text, Title } from '@mantine/core'
import { IconLayout } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { useCopyPage, useDeletePage, usePagePreview, usePageVersion, usePublishPage } from '../../api/pages'
import { useSite } from '../../api/public'
import type { PageVersion } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageFrame } from '../../components/PageFrame'
import { PageHeader } from '../../components/PageHeader'
import { errorText } from '../../lib/errors'
import { formatDateTime, formatSize } from '../../lib/format'
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
            loading={del.isPending} onConfirm={() => del.mutate(version.id, { onSuccess: () => navigate('/admin/pages') })}>
            {t('admin.pages.delete')}
          </ConfirmButton>
        )}
      </Group>
      {error && <Alert color="red">{errorText(t, error)}</Alert>}
    </Stack>
  )
}

/** The home page as visitors would see it with this version: our header, then the frame. */
function Preview({ id }: { id: number }) {
  const { t, i18n } = useTranslation()
  const preview = usePagePreview(id)
  const site = useSite()
  const text = useLocalized()
  if (preview.isPending) return <Loader />
  if (preview.isError) return <Alert color="red">{errorText(t, preview.error)}</Alert>
  return (
    <Card withBorder padding={0}>
      <Group justify="space-between" p="md">
        <Title order={2}>{text(site.data?.communityName) || t('home.title')}</Title>
        <Text size="sm" c="dimmed">{t('admin.pages.previewHint')}</Text>
      </Group>
      <PageFrame src={preview.data.url} lang={i18n.language} title={t('admin.pages.previewTitle')} />
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
      <Anchor component={Link} to="/admin/pages" size="sm">{t('admin.pages.back')}</Anchor>
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
      <Preview id={v.id} />
    </Stack>
  )
}
