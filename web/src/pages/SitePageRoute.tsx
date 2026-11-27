import { Center, Loader } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { useSite } from '../api/public'
import { PageFrame } from '../components/PageFrame'
import { SiteHeader } from '../components/SiteHeader'
import { useLocalized } from '../lib/localized'
import { NotFoundPage } from './NotFoundPage'

/** A published page at its own address (/contacts → contacts.html), or not found. */
export function SitePageRoute() {
  const { t, i18n } = useTranslation()
  const { page = '' } = useParams()
  const site = useSite()
  const text = useLocalized()
  const name = text(site.data?.communityName) || t('home.title')
  if (site.isPending) {
    return (
      <>
        <SiteHeader title={name} />
        <Center py="xl">
          <Loader />
        </Center>
      </>
    )
  }
  if (!site.data?.pages?.includes(`/${page}`)) return <NotFoundPage />
  return (
    <>
      <SiteHeader title={name} />
      <PageFrame src={`/site/${page}.html`} lang={i18n.language} title={name} />
    </>
  )
}
