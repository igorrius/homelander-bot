import { Anchor, Button, Group, Image, List, Loader, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useLogout, useMe, useRegisterOptions, useSite } from '../api/public'
import type { Me } from '../api/types'
import { HousesIllustration } from '../components/HousesIllustration'
import { PageFrame } from '../components/PageFrame'
import { PageShell } from '../components/PageShell'
import { useLocalized } from '../lib/localized'

function Account({ me }: { me: Me | null | undefined }) {
  const { t } = useTranslation()
  const logout = useLogout()
  if (me === undefined) return <Loader size="sm" />
  if (me === null) {
    return (
      <Group gap="sm">
        <Anchor component={Link} to="/register">
          {t('home.register')}
        </Anchor>
        <Button component={Link} to="/login">
          {t('home.login')}
        </Button>
      </Group>
    )
  }
  return (
    <Stack gap={4} align="flex-end">
      <Text fw={600}>{`${me.firstName} ${me.lastName}`.trim()}</Text>
      {me.residencies.length > 0 && (
        <List size="sm" listStyleType="none">
          {me.residencies.map((r) => (
            <List.Item key={r.id}>{t('home.apartment', { house: r.houseName, n: r.apartment })}</List.Item>
          ))}
        </List>
      )}
      <Group gap="sm">
        {me.isAdmin && (
          <Anchor component={Link} to="/admin">
            {t('home.admin')}
          </Anchor>
        )}
        <Button variant="default" size="xs" loading={logout.isPending} onClick={() => logout.mutate()}>
          {t('home.logout')}
        </Button>
      </Group>
    </Stack>
  )
}

export function HomePage() {
  const { t, i18n } = useTranslation()
  const site = useSite()
  const me = useMe()
  const options = useRegisterOptions()
  const text = useLocalized()
  const name = text(site.data?.communityName) || t('home.title')
  const welcome = text(site.data?.welcomeText) || t('home.welcomeDefault')
  const pictureUrl = site.data?.pictureUrl
  const pageUrl = site.data?.pageUrl
  const account = <Account me={me.isPending ? undefined : (me.data ?? null)} />
  if (pageUrl) {
    return (
      <>
        <PageShell title={name} actions={account} />
        {/* Full width below our header: the page owns its own layout. */}
        <PageFrame src={pageUrl} lang={i18n.language} title={name} />
      </>
    )
  }
  return (
    <PageShell title={name} actions={account}>
      <Stack gap="lg">
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
    </PageShell>
  )
}
