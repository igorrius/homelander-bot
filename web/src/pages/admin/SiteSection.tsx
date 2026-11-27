import { Alert, Loader, Stack, Tabs } from '@mantine/core'
import { IconHome, IconLayout, IconWorld } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { useAdminSite } from '../../api/admin'
import { PageHeader } from '../../components/PageHeader'
import { errorText } from '../../lib/errors'
import { SiteHomeTab } from './SiteHomeTab'
import { SitePagesTab, UploadZipButton } from './SitePagesTab'

type Tab = 'pages' | 'home'

/** The site in one place: published pages and drafts, and the built-in home page. */
export function SiteSection() {
  const { t } = useTranslation()
  const [params, setParams] = useSearchParams()
  const tab: Tab = params.get('tab') === 'home' ? 'home' : 'pages'
  const site = useAdminSite()
  return (
    <Stack>
      <PageHeader icon={IconWorld} title={t('admin.site.title')} description={t('admin.site.description')}
        actions={tab === 'pages' && <UploadZipButton />} />
      <Tabs value={tab} onChange={(v) => setParams({ tab: v ?? 'pages' }, { replace: true })} keepMounted={false}>
        <Tabs.List mb="lg">
          <Tabs.Tab value="pages" leftSection={<IconLayout size={16} />}>{t('admin.site.tabs.pages')}</Tabs.Tab>
          <Tabs.Tab value="home" leftSection={<IconHome size={16} />}>{t('admin.site.tabs.home')}</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="pages">
          <SitePagesTab />
        </Tabs.Panel>
        <Tabs.Panel value="home">
          {site.isPending ? <Loader /> : site.isError ? <Alert color="red">{errorText(t, site.error)}</Alert> : <SiteHomeTab site={site.data} />}
        </Tabs.Panel>
      </Tabs>
    </Stack>
  )
}
