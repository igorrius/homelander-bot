import { Alert, Button, Group, PinInput, Stack, Text, TextInput } from '@mantine/core'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useNavigate } from 'react-router'
import { useMe, useRequestCode, useVerifyCode } from '../api/public'
import { PageShell } from '../components/PageShell'
import { errorText, fieldErrors } from '../lib/errors'
import { useOnce } from '../lib/useOnce'

export function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
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
      navigate(user.isAdmin ? '/admin' : '/', { replace: true })
    } catch (err) {
      setCode('')
      throw err
    }
  })

  if (me.data && !verify.isSuccess) return <Navigate to={me.data.isAdmin ? '/admin' : '/'} replace />

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
