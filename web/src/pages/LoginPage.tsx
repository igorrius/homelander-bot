import { Alert, Button, Group, PinInput, Stack, Text, TextInput } from '@mantine/core'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { useMe, useRequestCode, useVerifyCode } from '../api/public'
import type { Me } from '../api/types'
import { PageShell } from '../components/PageShell'
import { errorText, fieldErrors } from '../lib/errors'
import { safeNext } from '../lib/redirect'
import { useOnce } from '../lib/useOnce'

/** Where a user lands after login without a next address: admins in the panel, everyone else in the cabinet. */
function home(u: Me): string {
  return u.isAdmin ? '/admin' : '/my'
}

export function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const next = safeNext(params.get('next'))
  const linkExpired = params.get('link') === 'expired'
  const me = useMe()
  const requestCode = useRequestCode()
  const verify = useVerifyCode()
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [step, setStep] = useState<'phone' | 'code'>('phone')

  const sendCode = useOnce(async () => {
    verify.reset()
    await requestCode.mutateAsync({ phone: phone.trim() })
    setCode('')
    setStep('code')
  })
  const submitCode = useOnce(async () => {
    try {
      const { user } = await verify.mutateAsync({ phone: phone.trim(), code })
      navigate(next ?? home(user), { replace: true })
    } catch (err) {
      setCode('')
      throw err
    }
  })

  if (me.data && !verify.isSuccess) return <Navigate to={next ?? home(me.data)} replace />

  const onPhone = (e: FormEvent) => {
    e.preventDefault()
    sendCode()
  }
  const onCode = (e: FormEvent) => {
    e.preventDefault()
    if (code.length === 6) submitCode()
  }
  const phoneError = fieldErrors(t, requestCode.error).phone
  const verifyError = verify.error ? (verify.error.code === 'invalid_code' ? t('errors.invalid_code') : errorText(t, verify.error)) : null

  return (
    <PageShell title={t('login.title')}>
      <Stack maw={420}>
        {linkExpired && (
          <Alert color="yellow" title={t('login.linkExpired')}>
            {t('login.linkHint')}
          </Alert>
        )}
        {step === 'phone' ? (
          <form onSubmit={onPhone}>
            <Stack>
              <TextInput
                label={t('login.phone')}
                placeholder={t('login.phonePlaceholder')}
                type="tel"
                autoComplete="tel"
                value={phone}
                onChange={(e) => setPhone(e.currentTarget.value)}
                error={phoneError}
              />
              {requestCode.error && !phoneError && <Alert color="red">{errorText(t, requestCode.error)}</Alert>}
              <Button type="submit" loading={requestCode.isPending} disabled={!phone.trim()}>
                {t('login.sendCode')}
              </Button>
            </Stack>
          </form>
        ) : (
          <form onSubmit={onCode}>
            <Stack>
              <Text>{t('login.codeSent')}</Text>
              <PinInput
                length={6}
                type="number"
                oneTimeCode
                ariaLabel={t('login.code')}
                value={code}
                onChange={setCode}
                error={!!verifyError}
                autoFocus
              />
              {verifyError && (
                <Text c="red" size="sm">
                  {verifyError}
                </Text>
              )}
              {requestCode.error && <Alert color="red">{errorText(t, requestCode.error)}</Alert>}
              <Button type="submit" loading={verify.isPending} disabled={code.length !== 6}>
                {t('login.submit')}
              </Button>
              <Group justify="space-between">
                <Button variant="subtle" size="xs" onClick={() => setStep('phone')}>
                  {t('login.changePhone')}
                </Button>
                <Button variant="subtle" size="xs" loading={requestCode.isPending} onClick={() => sendCode()}>
                  {t('login.resend')}
                </Button>
              </Group>
            </Stack>
          </form>
        )}
      </Stack>
    </PageShell>
  )
}
