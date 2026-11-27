import { useTranslation } from 'react-i18next'
import { useRegisterOptions, useSite } from '../api/public'
import { usePublicData } from '../api/publicData'
import { BuiltInHome } from '../components/BuiltInHome'
import { PageFrame } from '../components/PageFrame'
import { SiteHeader } from '../components/SiteHeader'
import { useLocalized } from '../lib/localized'

export function HomePage() {
  const { t, i18n } = useTranslation()
  const site = useSite()
  const options = useRegisterOptions()
  const text = useLocalized()
  const name = text(site.data?.communityName) || t('home.title')
  const welcome = text(site.data?.welcomeText) || t('home.welcomeDefault')
  const pageUrl = site.data?.pageUrl
  const homeItems = site.data?.homeItems ?? []
  const publicData = usePublicData(!pageUrl && homeItems.length > 0)
  if (pageUrl) {
    return (
      <>
        <SiteHeader title={name} />
        {/* Full width below our header: the page owns its own layout. */}
        <PageFrame src={pageUrl} lang={i18n.language} title={name} />
      </>
    )
  }
  return (
    <>
      <SiteHeader title={name} />
      <BuiltInHome name={name} welcome={welcome} houses={site.isPending ? undefined : options.data?.houses} homeItems={homeItems}
        data={publicData.data} />
    </>
  )
}
