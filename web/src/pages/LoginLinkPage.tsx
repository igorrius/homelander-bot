import { Alert, Button, Center, Group, Loader, Stack, Text } from '@mantine/core'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { ApiError } from '../api/client'
import { useLoginLink } from '../api/cabinet'
import { useMe } from '../api/public'
import { PageShell } from '../components/PageShell'
import { errorText } from '../lib/errors'
import { fullName } from '../lib/format'
import { safeNext } from '../lib/redirect'

const EXPIRED = '/login?link=expired'

/**
 * Opens a one-time login link from the bot (/login/link?t=…). Loading this
 * page spends nothing (a messenger may fetch it for a preview); the POST it
 * sends once does, then it opens the cabinet page the link names. When the
 * browser is signed in as someone else, the server leaves the link unspent
 * and the person chooses: switch to the link's account, or stay.
 */
export function LoginLinkPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const token = useSearchParams()[0].get('t')
  const me = useMe().data
  const { mutateAsync, error, isPending } = useLoginLink()
  const [otherUser, setOtherUser] = useState(false)

  const redeem = useCallback(
    (switchUser: boolean) => {
      if (!token) return
      mutateAsync(switchUser ? { token, switch: true } : { token }).then(
        ({ next }) => navigate(safeNext(next) ?? '/my', { replace: true }),
        (err: unknown) => {
          if (!(err instanceof ApiError)) return
          if (err.code === 'token_invalid') navigate(EXPIRED, { replace: true })
          else if (err.code === 'link_other_user') setOtherUser(true)
        },
      )
    },
    [token, mutateAsync, navigate],
  )

  // Effects run twice under StrictMode; a second POST would spend the link
  // for nothing and report it expired.
  const sent = useRef(false)
  useEffect(() => {
    if (sent.current) return
    sent.current = true
    if (!token) navigate(EXPIRED, { replace: true })
    else redeem(false)
  }, [token, navigate, redeem])

  if (otherUser) {
    return (
      <PageShell title={t('login.title')}>
        <Stack maw={420}>
          <Text>{t('login.linkOtherUser', { name: me ? fullName(me) : '' })}</Text>
          <Group>
            <Button loading={isPending} onClick={() => redeem(true)}>
              {t('login.linkContinue')}
            </Button>
            <Button variant="default" disabled={isPending} onClick={() => navigate('/my', { replace: true })}>
              {t('login.linkStay')}
            </Button>
          </Group>
        </Stack>
      </PageShell>
    )
  }
  if (error && error.code !== 'token_invalid' && error.code !== 'link_other_user') {
    return (
      <PageShell title={t('login.title')}>
        <Stack maw={420}>
          <Alert color="red">{errorText(t, error)}</Alert>
          <Button component={Link} to="/login" variant="default">
            {t('home.login')}
          </Button>
        </Stack>
      </PageShell>
    )
  }
  return (
    <Center h="100vh">
      <Stack align="center" gap="sm">
        <Loader />
        <Text c="dimmed">{t('login.linkOpening')}</Text>
      </Stack>
    </Center>
  )
}
