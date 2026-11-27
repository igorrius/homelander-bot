import { Alert, Button, Card, Group, Loader, Select, Stack, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useState, type FormEvent } from 'react'
import { IconWorld } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useAdminSite, useSaveSite } from '../../api/admin'
import type { Locale, Site } from '../../api/types'
import { LocalizedTextInputs } from '../../components/LocalizedTextInputs'
import { supportedLocales } from '../../i18n'
import { errorText, useApiErrors } from '../../lib/errors'
import { cleanLocalized } from '../../lib/localized'
import { PageHeader } from '../../components/PageHeader'

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
      { communityName: cleanLocalized(communityName), welcomeText: cleanLocalized(welcomeText), defaultLocale, homeItems: site.homeItems },
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

export function SitePage() {
  const { t } = useTranslation()
  const site = useAdminSite()
  if (site.isPending) return <Loader />
  if (site.isError) return <Alert color="red">{errorText(t, site.error)}</Alert>
  return (
    <Stack>
      <PageHeader icon={IconWorld} title={t('admin.site.title')} />
      <Texts site={site.data} />
    </Stack>
  )
}
