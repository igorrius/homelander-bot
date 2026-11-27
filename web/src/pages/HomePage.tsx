import { Container, Image, Loader, Stack, Text, Title } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useRegisterOptions, useSite } from '../api/public'
import { HousesIllustration } from '../components/HousesIllustration'
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
  const pictureUrl = site.data?.pictureUrl
  const pageUrl = site.data?.pageUrl
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
      <Container size="md" py="xl">
        <Stack gap="lg">
          <Title order={1}>{name}</Title>
          {site.isPending ? (
            <Loader />
          ) : pictureUrl ? (
            <Image src={pictureUrl} alt={name} radius="md" mah={480} fit="contain" />
          ) : (
            <HousesIllustration houses={options.data?.houses ?? []} />
          )}
          {welcome.split(/\n\s*\n/).map((p, i) => (
            <Text key={i} style={{ whiteSpace: 'pre-line' }}>
              {p}
            </Text>
          ))}
        </Stack>
      </Container>
    </>
  )
}
