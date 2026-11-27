import { Alert, Button, Card, Group, Loader, Stack, Text } from '@mantine/core'
import { IconPlugConnected } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useSearchParams } from 'react-router'
import { useAuthorize, useAuthorizeInfo } from '../api/oauth'
import { useMe } from '../api/public'
import { PageShell } from '../components/PageShell'
import { errorText } from '../lib/errors'
import { redirect } from '../lib/redirect'

/**
 * The OAuth consent screen: an AI agent asks to act as the signed-in admin
 * through MCP. Shows the agent's self-reported name and where it will send
 * the answer, so an admin can spot a client that only claims to be Claude.
 */
export function OAuthAuthorizePage() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const location = useLocation()
  const me = useMe()
  const clientId = params.get('client_id') ?? ''
  const redirectUri = params.get('redirect_uri') ?? ''
  const wellFormed = params.get('response_type') === 'code' && clientId !== '' && redirectUri !== ''
  const info = useAuthorizeInfo(wellFormed ? clientId : '', redirectUri, !!me.data)
  const authorize = useAuthorize()

  const answer = (approve: boolean) =>
    authorize.mutate(
      {
        clientId, redirectUri, approve,
        codeChallenge: params.get('code_challenge') ?? '',
        codeChallengeMethod: params.get('code_challenge_method') ?? '',
        state: params.get('state') ?? '',
        resource: params.get('resource') ?? '',
      },
      { onSuccess: (r) => redirect.to(r.redirect) },
    )

  let body
  if (!wellFormed || info.isError) {
    body = <Alert color="red">{t('oauth.invalid')}</Alert>
  } else if (info.isPending || me.isPending) {
    body = <Loader />
  } else if (!info.data.signedIn) {
    body = (
      <Stack>
        <Text>{t('oauth.loginFirst', { client: info.data.clientName })}</Text>
        <Group>
          <Button component={Link} to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`}>
            {t('oauth.login')}
          </Button>
        </Group>
      </Stack>
    )
  } else {
    const { clientName, redirectHost, allowed, reason } = info.data
    body = (
      <Stack>
        <Text size="lg">{t('oauth.question', { client: clientName })}</Text>
        <Text c="dimmed">{t('oauth.redirectHost', { host: redirectHost })}</Text>
        {allowed ? (
          <Text size="sm">{t('oauth.explain')}</Text>
        ) : (
          <Alert color="yellow">{t(`oauth.reason.${reason ?? 'not_admin'}`)}</Alert>
        )}
        {authorize.error && <Alert color="red">{errorText(t, authorize.error)}</Alert>}
        <Group>
          {allowed && (
            <Button leftSection={<IconPlugConnected size={16} />} loading={authorize.isPending} onClick={() => answer(true)}>
              {t('oauth.allow')}
            </Button>
          )}
          <Button variant="default" disabled={authorize.isPending} onClick={() => answer(false)}>
            {t('common.cancel')}
          </Button>
        </Group>
      </Stack>
    )
  }
  return (
    <PageShell title={t('oauth.title')}>
      <Card withBorder maw={560}>
        {body}
      </Card>
    </PageShell>
  )
}
