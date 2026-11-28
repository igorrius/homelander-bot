import { Alert, Button, Center, Loader, Stack, Text } from '@mantine/core'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { ApiError } from '../api/client'
import { useLoginLink } from '../api/cabinet'
import { PageShell } from '../components/PageShell'
import { errorText } from '../lib/errors'
import { safeNext } from '../lib/redirect'

const EXPIRED = '/login?link=expired'

/**
 * Opens a one-time login link from the bot (/login/link?t=…). Loading this
 * page spends nothing (a messenger may fetch it for a preview); the POST it
 * sends once does, then it opens the cabinet page the link names.
 */
export function LoginLinkPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const token = useSearchParams()[0].get('t')
  const { mutateAsync, error } = useLoginLink()
  // Effects run twice under StrictMode; a second POST would spend the link
  // for nothing and report it expired.
  const sent = useRef(false)
  useEffect(() => {
    if (sent.current) return
    sent.current = true
    if (!token) {
      navigate(EXPIRED, { replace: true })
      return
    }
    mutateAsync({ token }).then(
      ({ next }) => navigate(safeNext(next) ?? '/my', { replace: true }),
      (err: unknown) => {
        if (err instanceof ApiError && err.code === 'token_invalid') navigate(EXPIRED, { replace: true })
      },
    )
  }, [token, navigate, mutateAsync])

  if (error && error.code !== 'token_invalid') {
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
