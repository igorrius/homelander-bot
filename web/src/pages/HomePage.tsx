import { Box, Center, Container, Image, Loader, Stack, Text, Title, useComputedColorScheme } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useRegisterOptions, useSite } from '../api/public'
import { Quarter } from '../components/Quarter'
import { PageFrame } from '../components/PageFrame'
import { SiteHeader } from '../components/SiteHeader'
import { useLocalized } from '../lib/localized'

const HERO_LIGHT = 'linear-gradient(180deg, #eef7f1 0%, #e4f0f6 70%, var(--mantine-color-body) 100%)'
const HERO_DARK = 'linear-gradient(180deg, #111a22 0%, #16222d 70%, var(--mantine-color-body) 100%)'

export function HomePage() {
  const { t, i18n } = useTranslation()
  const site = useSite()
  const options = useRegisterOptions()
  const text = useLocalized()
  const name = text(site.data?.communityName) || t('home.title')
  const welcome = text(site.data?.welcomeText) || t('home.welcomeDefault')
  const pictureUrl = site.data?.pictureUrl
  const pageUrl = site.data?.pageUrl
  const dark = useComputedColorScheme('light') === 'dark'
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
      <Box component="section" pt={48} pb={40} style={{ background: dark ? HERO_DARK : HERO_LIGHT }}>
        <Container size="lg">
          <Stack gap="md" align="center" ta="center">
            <Title order={1} maw={920} style={{ fontSize: 'clamp(1.75rem, 1.1rem + 2.6vw, 2.9rem)', lineHeight: 1.15, letterSpacing: '-0.02em', textWrap: 'balance' }}>
              {name}
            </Title>
            <Stack gap={6} maw={640}>
              {welcome.split(/\n\s*\n/).map((p, i) => (
                <Text key={i} size="lg" c="dimmed" style={{ whiteSpace: 'pre-line', textWrap: 'pretty' }}>
                  {p}
                </Text>
              ))}
            </Stack>
          </Stack>
          <Box mt={36}>
            {site.isPending || options.isPending ? (
              <Center h={240}>
                <Loader />
              </Center>
            ) : pictureUrl ? (
              <Image src={pictureUrl} alt={name} radius="md" mah={480} fit="contain" />
            ) : (
              <Quarter houses={options.data?.houses ?? []} />
            )}
          </Box>
        </Container>
      </Box>
    </>
  )
}
