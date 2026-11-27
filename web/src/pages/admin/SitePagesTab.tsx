import { Accordion, Alert, Box, Button, Card, FileInput, Group, Loader, Modal, Stack, Text, TextInput, ThemeIcon, Title } from '@mantine/core'
import { IconArchive, IconExternalLink, IconFileUpload, IconPencil, IconUpload, IconWorld } from '@tabler/icons-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { useDeletePage, usePageVersions, usePublishPage, useUnpublishPage, useUploadPage } from '../../api/pages'
import type { PageVersion } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { errorText, useApiErrors } from '../../lib/errors'
import { formatDateTime } from '../../lib/format'
import { Author, PageChips, VersionCard } from './VersionCard'

// The server takes up to 25 MiB; checked here too, since it may cut an oversized upload off.
const maxZipBytes = 25 * 1024 * 1024
const namedReasons = ['type', 'invalid', 'too_large', 'reserved_page_name', 'invalid_page_name']

/** Upload a zip as a new draft; opens the draft's preview when it is stored. */
export function UploadZipButton() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const upload = useUploadPage()
  const [opened, setOpened] = useState(false)
  const [note, setNote] = useState('')
  const [tooBig, setTooBig] = useState(false)
  const { message, fields } = useApiErrors(upload.error)
  // Page files get their own wording ("too_many" is shared with table rows), naming the refused file.
  const reason = upload.error?.fields.files
  const path = upload.error?.details.path
  const fileError = reason === 'too_many' ? t('admin.pages.tooManyFiles')
    : path && reason && namedReasons.includes(reason) ? t(`admin.pages.fileError.${reason}`, { path })
      : (fields.files ?? fields.zip)
  const close = () => {
    setOpened(false)
    upload.reset()
    setTooBig(false)
  }
  return (
    <>
      <Button leftSection={<IconUpload size={16} />} variant="default" onClick={() => setOpened(true)}>
        {t('admin.site.uploadZip')}
      </Button>
      <Modal opened={opened} onClose={close} title={t('admin.site.uploadTitle')} centered>
        <Stack>
          <TextInput label={t('admin.pages.note')} value={note} onChange={(e) => setNote(e.currentTarget.value)} error={fields.note} maxLength={200} />
          <FileInput label={t('admin.pages.upload')} description={t('admin.pages.uploadHint')} accept=".zip,application/zip"
            leftSection={<IconFileUpload size={16} />} value={null} clearable={false} disabled={upload.isPending}
            error={tooBig ? t('validation.too_large') : fileError}
            onChange={(f) => {
              upload.reset()
              setTooBig(!!f && f.size > maxZipBytes)
              if (f && f.size <= maxZipBytes) {
                upload.mutate({ file: f, note: note.trim() }, { onSuccess: (v) => navigate(`/admin/pages/${v.id}`) })
              }
            }} />
          {upload.isPending && <Loader size="sm" />}
          {message && <Alert color="red">{message}</Alert>}
        </Stack>
      </Modal>
    </>
  )
}

function SectionTitle({ color, icon, title, hint }: { color: string; icon: ReactNode; title: string; hint?: string }) {
  return (
    <Group gap="sm" mb="sm">
      <ThemeIcon variant="light" color={color} radius="md" size={30}>{icon}</ThemeIcon>
      <div>
        <Title order={3} size="h5">{title}</Title>
        {hint && <Text size="xs" c="dimmed">{hint}</Text>}
      </div>
    </Group>
  )
}

function Live({ version }: { version: PageVersion | undefined }) {
  const { t, i18n } = useTranslation()
  const unpublish = useUnpublishPage()
  if (!version) {
    return (
      <Card withBorder radius="lg" padding="lg" style={{ borderStyle: 'dashed' }} data-testid="site-live">
        <Group gap="md" wrap="nowrap" align="flex-start">
          <ThemeIcon variant="light" color="gray" size={40} radius="md"><IconWorld size={22} /></ThemeIcon>
          <div>
            <Text fw={600}>{t('admin.site.liveEmpty')}</Text>
            <Text size="sm" c="dimmed">{t('admin.site.liveEmptyHint')}</Text>
          </div>
        </Group>
      </Card>
    )
  }
  return (
    <Card withBorder radius="lg" padding="lg" data-testid="site-live"
      style={{ borderColor: 'var(--mantine-color-green-filled)', background: 'var(--mantine-color-green-light)' }}>
      <Group justify="space-between" align="flex-start" gap="md">
        <Stack gap={8} style={{ minWidth: 0, flex: '1 1 320px' }}>
          <Group gap="xs">
            <Box w={10} h={10} bg="green" style={{ borderRadius: 99, boxShadow: '0 0 0 4px var(--mantine-color-green-light)' }} />
            <Text fw={700} c="green" tt="uppercase" size="xs" style={{ letterSpacing: '0.06em' }}>{t('admin.site.live')}</Text>
            <Text size="xs" c="dimmed">#{version.id}</Text>
          </Group>
          <Text fw={650} size="lg">{version.note || t('admin.site.version', { id: version.id })}</Text>
          <Author version={version} />
          {version.publishedAt && (
            <Text size="xs" c="dimmed">
              {t('admin.site.publishedBy', { at: formatDateTime(version.publishedAt, i18n.language), name: version.publishedBy?.name ?? '—' })}
            </Text>
          )}
          <PageChips version={version} linked />
        </Stack>
        <Stack gap="xs" align="flex-end">
          <Button component="a" href="/" target="_blank" rel="noopener" color="green" rightSection={<IconExternalLink size={16} />}>
            {t('admin.site.openSite')}
          </Button>
          <Group gap="xs">
            <Button component={Link} to={`/admin/pages/${version.id}`} variant="default" size="xs">{t('admin.site.preview')}</Button>
            <ConfirmButton variant="subtle" color="gray" size="xs" message={t('admin.pages.unpublishConfirm')} loading={unpublish.isPending}
              onConfirm={() => unpublish.mutate()}>
              {t('admin.pages.unpublish')}
            </ConfirmButton>
          </Group>
        </Stack>
      </Group>
      {unpublish.error && <Alert color="red" mt="sm">{errorText(t, unpublish.error)}</Alert>}
    </Card>
  )
}

function DeleteButton({ version }: { version: PageVersion }) {
  const { t } = useTranslation()
  const del = useDeletePage()
  return (
    <ConfirmButton color="red" variant="subtle" size="xs" message={t('admin.pages.deleteConfirm', { id: version.id })} loading={del.isPending}
      onConfirm={() => del.mutate(version.id)}>
      {t('admin.pages.delete')}
    </ConfirmButton>
  )
}

function RestoreButton({ version }: { version: PageVersion }) {
  const { t } = useTranslation()
  const publish = usePublishPage()
  return (
    <ConfirmButton variant="light" size="xs" message={t('admin.site.restoreConfirm', { id: version.id })} loading={publish.isPending}
      onConfirm={() => publish.mutate(version.id)}>
      {t('admin.site.restore')}
    </ConfirmButton>
  )
}

/** The Pages tab: what is live, drafts waiting for review, and the archive. */
export function SitePagesTab() {
  const { t } = useTranslation()
  const versions = usePageVersions()
  if (versions.isPending) return <Loader />
  if (versions.isError) return <Alert color="red">{errorText(t, versions.error)}</Alert>
  const live = versions.data.find((v) => v.status === 'published')
  const drafts = versions.data.filter((v) => v.status === 'draft')
  const archived = versions.data.filter((v) => v.status === 'archived')
  return (
    <Stack gap="xl">
      <Live version={live} />
      <Box data-testid="site-drafts">
        <SectionTitle color="yellow" icon={<IconPencil size={16} />} title={`${t('admin.site.drafts')}${drafts.length ? ` · ${drafts.length}` : ''}`}
          hint={drafts.length ? t('admin.site.draftsHint') : undefined} />
        {drafts.length === 0 ? (
          <Text c="dimmed" size="sm">{t('admin.site.draftsEmpty')}</Text>
        ) : (
          <Stack gap="sm">
            {drafts.map((v) => (
              <Box key={v.id} style={{ borderLeft: '3px solid var(--mantine-color-yellow-filled)', borderRadius: 'var(--mantine-radius-md)' }}>
                <VersionCard version={v} actions={
                  <>
                    <DeleteButton version={v} />
                    <Button component={Link} to={`/admin/pages/${v.id}`} size="sm">{t('admin.site.previewPublish')}</Button>
                  </>
                } />
              </Box>
            ))}
          </Stack>
        )}
      </Box>
      {archived.length > 0 && (
        <Accordion variant="contained" radius="md">
          <Accordion.Item value="archive">
            <Accordion.Control icon={<IconArchive size={18} />}>{t('admin.site.archive', { count: archived.length })}</Accordion.Control>
            <Accordion.Panel>
              <Stack gap="sm">
                {archived.map((v) => (
                  <VersionCard key={v.id} version={v} actions={
                    <>
                      <DeleteButton version={v} />
                      <Button component={Link} to={`/admin/pages/${v.id}`} variant="default" size="xs">{t('admin.site.preview')}</Button>
                      <RestoreButton version={v} />
                    </>
                  } />
                ))}
              </Stack>
            </Accordion.Panel>
          </Accordion.Item>
        </Accordion>
      )}
    </Stack>
  )
}
