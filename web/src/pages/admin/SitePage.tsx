import { Alert, Button, Card, FileInput, Group, Image, Loader, Select, Stack, Text, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useState, type FormEvent } from 'react'
import { IconWorld } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useAdminSite, useDeletePicture, useHouses, useSaveSite, useUploadPicture } from '../../api/admin'
import type { Locale, Site } from '../../api/types'
import { ConfirmButton } from '../../components/ConfirmButton'
import { HousesIllustration } from '../../components/HousesIllustration'
import { LocalizedTextInputs } from '../../components/LocalizedTextInputs'
import { supportedLocales } from '../../i18n'
import { errorText, useApiErrors } from '../../lib/errors'
import { cleanLocalized } from '../../lib/localized'
import { PageHeader } from '../../components/PageHeader'

const maxPictureBytes = 5 << 20

function Texts({ site }: { site: Site }) {
  const { t } = useTranslation()
  const save = useSaveSite()
  const [communityName, setName] = useState(site.communityName)
  const [welcomeText, setWelcome] = useState(site.welcomeText)
  const [defaultLocale, setDefaultLocale] = useState<Locale>(site.defaultLocale)
  const { message, fields } = useApiErrors(save.error)
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    save.mutate(
      { communityName: cleanLocalized(communityName), welcomeText: cleanLocalized(welcomeText), defaultLocale },
      { onSuccess: () => notifications.show({ message: t('common.saved'), color: 'green' }) },
    )
  }
  return (
    <Card withBorder>
      <form onSubmit={onSubmit}>
        <Stack>
          <Title order={3}>{t('admin.site.texts')}</Title>
          <LocalizedTextInputs label={t('admin.site.communityName')} value={communityName} onChange={setName} error={fields.communityName} />
          <LocalizedTextInputs label={t('admin.site.welcomeText')} value={welcomeText} onChange={setWelcome} error={fields.welcomeText} multiline />
          <Select label={t('admin.site.defaultLocale')} value={defaultLocale} onChange={(v) => v && setDefaultLocale(v as Locale)} allowDeselect={false}
            data={supportedLocales.map((l) => ({ value: l, label: t(`language.${l}`) }))} error={fields.defaultLocale} w={220} />
          {message && <Alert color="red">{message}</Alert>}
          <Group>
            <Button type="submit" loading={save.isPending}>
              {t('admin.site.save')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Card>
  )
}

function Picture({ site }: { site: Site }) {
  const { t } = useTranslation()
  const houses = useHouses()
  const upload = useUploadPicture()
  const remove = useDeletePicture()
  const [tooBig, setTooBig] = useState(false)
  const { message, fields } = useApiErrors(upload.error ?? remove.error)
  return (
    <Card withBorder>
      <Stack>
        <Title order={3}>{t('admin.site.picture')}</Title>
        {site.pictureUrl ? (
          <Image src={site.pictureUrl} alt={t('admin.site.picture')} mah={320} fit="contain" radius="md" />
        ) : (
          <>
            <Text size="sm" c="dimmed">{t('admin.site.noPicture')}</Text>
            <HousesIllustration houses={houses.data ?? []} />
          </>
        )}
        <Group align="flex-end">
          <FileInput label={t('admin.site.upload')} description={t('admin.site.pictureHint')} accept="image/png,image/jpeg,image/webp"
            value={null} onChange={(f) => {
              remove.reset()
              upload.reset()
              // Checked here too: the server may cut an oversized upload off before it can answer.
              setTooBig(!!f && f.size > maxPictureBytes)
              if (f && f.size <= maxPictureBytes) {
                upload.mutate(f, { onSuccess: () => notifications.show({ message: t('common.saved'), color: 'green' }) })
              }
            }}
            error={tooBig ? t('validation.size') : fields.picture} disabled={upload.isPending} w={280} clearable={false} />
          {upload.isPending && <Loader size="sm" />}
          {site.pictureUrl && (
            <ConfirmButton color="red" variant="light" loading={remove.isPending} message={t('admin.site.removePictureConfirm')}
              onConfirm={() => {
                upload.reset()
                remove.mutate()
              }}>
              {t('admin.site.removePicture')}
            </ConfirmButton>
          )}
        </Group>
        {message && <Alert color="red">{message}</Alert>}
      </Stack>
    </Card>
  )
}

export function SitePage() {
  const { t } = useTranslation()
  const site = useAdminSite()
  if (site.isPending) return <Loader />
  if (site.isError) return <Alert color="red">{errorText(t, site.error)}</Alert>
  return (
    <Stack>
      <PageHeader icon={IconWorld} title={t('admin.site.title')} />
      <Texts site={site.data} />
      <Picture site={site.data} />
    </Stack>
  )
}
