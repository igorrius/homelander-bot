import { Alert, Anchor, Badge, FileInput, Group, Loader, Stack, Table, Text, TextInput } from '@mantine/core'
import { IconLayout, IconUpload } from '@tabler/icons-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { usePageVersions, useUnpublishPage, useUploadPage } from '../../api/pages'
import { ConfirmButton } from '../../components/ConfirmButton'
import { PageHeader } from '../../components/PageHeader'
import { errorText, useApiErrors } from '../../lib/errors'
import { formatDateTime, formatSize } from '../../lib/format'
import { PageStatusBadge } from './PageStatusBadge'

// The server takes up to 25 MiB; checked here too, since it may cut an oversized upload off.
const maxZipBytes = 25 * 1024 * 1024

function Upload() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const upload = useUploadPage()
  const [note, setNote] = useState('')
  const [tooBig, setTooBig] = useState(false)
  const { message, fields } = useApiErrors(upload.error)
  // Page files get their own wording ("too_many" is shared with table rows), naming the refused file.
  const reason = upload.error?.fields.files
  const path = upload.error?.details.path
  const fileError = reason === 'too_many' ? t('admin.pages.tooManyFiles')
    : path && reason && ['type', 'invalid', 'too_large'].includes(reason) ? t(`admin.pages.fileError.${reason}`, { path })
      : (fields.files ?? fields.zip)
  return (
    <Stack gap="xs">
      <Group align="flex-start">
        <TextInput label={t('admin.pages.note')} value={note} onChange={(e) => setNote(e.currentTarget.value)}
          error={fields.note} maxLength={200} w={280} />
        <FileInput label={t('admin.pages.upload')} description={t('admin.pages.uploadHint')} accept=".zip,application/zip"
          leftSection={<IconUpload size={16} />} value={null} clearable={false} disabled={upload.isPending} w={280}
          error={tooBig ? t('validation.too_large') : fileError}
          onChange={(f) => {
            upload.reset()
            setTooBig(!!f && f.size > maxZipBytes)
            if (f && f.size <= maxZipBytes) {
              upload.mutate({ file: f, note: note.trim() }, { onSuccess: (v) => navigate(`/admin/pages/${v.id}`) })
            }
          }} />
        {upload.isPending && <Loader size="sm" mt="xl" />}
      </Group>
      {message && <Alert color="red">{message}</Alert>}
    </Stack>
  )
}

/** Home page versions: upload, list, and the way back to the built-in page. */
export function PagesPage() {
  const { t, i18n } = useTranslation()
  const versions = usePageVersions()
  const unpublish = useUnpublishPage()
  const published = versions.data?.some((v) => v.status === 'published')
  return (
    <Stack>
      <PageHeader icon={IconLayout} title={t('admin.pages.title')} description={t('admin.pages.description')}
        actions={published && (
          <ConfirmButton variant="default" message={t('admin.pages.unpublishConfirm')} loading={unpublish.isPending}
            onConfirm={() => unpublish.mutate()}>
            {t('admin.pages.unpublish')}
          </ConfirmButton>
        )} />
      <Upload />
      {unpublish.error && <Alert color="red">{errorText(t, unpublish.error)}</Alert>}
      {versions.isPending ? (
        <Loader />
      ) : versions.isError ? (
        <Alert color="red">{errorText(t, versions.error)}</Alert>
      ) : versions.data.length === 0 ? (
        <Text c="dimmed">{t('admin.pages.empty')}</Text>
      ) : (
        <Table striped>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('admin.pages.version')}</Table.Th>
              <Table.Th>{t('admin.pages.statusLabel')}</Table.Th>
              <Table.Th>{t('admin.pages.files')}</Table.Th>
              <Table.Th>{t('admin.pages.created')}</Table.Th>
              <Table.Th>{t('admin.pages.published')}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {versions.data.map((v) => (
              <Table.Tr key={v.id}>
                <Table.Td>
                  <Anchor component={Link} to={`/admin/pages/${v.id}`}>#{v.id}</Anchor>
                  {v.note && <Text size="xs" c="dimmed">{v.note}</Text>}
                </Table.Td>
                <Table.Td><PageStatusBadge status={v.status} /></Table.Td>
                <Table.Td>
                  <Text size="sm">{t('admin.pages.fileCount', { count: v.files.length })}</Text>
                  <Text size="xs" c="dimmed">{formatSize(v.totalSize)}</Text>
                </Table.Td>
                <Table.Td>
                  <Text size="sm">{formatDateTime(v.createdAt, i18n.language)}</Text>
                  {v.createdBy && <Text size="xs" c="dimmed">{v.createdBy.name}</Text>}
                  {v.createdVia && (
                    <Badge size="xs" variant="light" color="grape" title={v.createdVia}>
                      {t('admin.pages.viaMcp')}
                    </Badge>
                  )}
                </Table.Td>
                <Table.Td>
                  {v.publishedAt && <Text size="sm">{formatDateTime(v.publishedAt, i18n.language)}</Text>}
                  {v.publishedBy && <Text size="xs" c="dimmed">{v.publishedBy.name}</Text>}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  )
}
