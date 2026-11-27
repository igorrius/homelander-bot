import { Alert, Button, Checkbox, Loader, Select, Stack, Text, TextInput, Title } from '@mantine/core'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import { usePrefill, useRegister, useRegisterOptions, useSite } from '../api/public'
import { PageShell } from '../components/PageShell'
import { QrLink } from '../components/QrLink'
import { apartmentOptions } from '../lib/apartments'
import { useApiErrors } from '../lib/errors'
import { useLocalized } from '../lib/localized'
import { useOnce } from '../lib/useOnce'

function Done({ botUrl }: { botUrl: string }) {
  const { t } = useTranslation()
  return (
    <Stack maw={520}>
      <Title order={2}>{t('register.doneTitle')}</Title>
      <Text>{t('register.doneText')}</Text>
      <Button component="a" href={botUrl} target="_blank" rel="noopener noreferrer" w="fit-content">
        {t('register.openBot')}
      </Button>
      <Text size="sm" c="dimmed">
        {t('register.qrHint')}
      </Text>
      <QrLink url={botUrl} />
    </Stack>
  )
}

/** The token of a registration link from the bot ("#t=…"), or null. */
function tokenFromHash(hash: string): string | null {
  const v = new URLSearchParams(hash.replace(/^#/, '')).get('t')
  return v ? v : null
}

const isTokenInvalid = (err: unknown) => err instanceof ApiError && err.code === 'token_invalid'

function LinkExpired({ botUrl }: { botUrl: string }) {
  const { t } = useTranslation()
  return (
    <Alert color="orange">
      <Stack gap="xs">
        <Text size="sm">{t('register.linkExpired')}</Text>
        <Button component="a" href={botUrl} target="_blank" rel="noopener noreferrer" size="xs" w="fit-content">
          {t('register.openBot')}
        </Button>
      </Stack>
    </Alert>
  )
}

export function RegisterPage() {
  const { t, i18n } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const options = useRegisterOptions()
  const site = useSite()
  const register = useRegister()
  const text = useLocalized()
  // The token is read once and taken out of the address bar, so it is not
  // kept in history or shared by copying the URL.
  const [token, setToken] = useState(() => tokenFromHash(location.hash))
  const [expired, setExpired] = useState(false)
  const prefill = usePrefill(token)
  const [phone, setPhone] = useState('')
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [houseId, setHouseId] = useState<string | null>(null)
  const [apartment, setApartment] = useState<string | null>(null)
  const [statusId, setStatusId] = useState<string | null>(null)
  const [consent, setConsent] = useState(false)
  const { message, fields } = useApiErrors(isTokenInvalid(register.error) ? null : register.error)

  useEffect(() => {
    if (location.hash) void navigate({ pathname: location.pathname, search: location.search }, { replace: true })
  }, [location.hash, location.pathname, location.search, navigate])
  useEffect(() => {
    if (prefill.data) setPhone(prefill.data.phone)
  }, [prefill.data])
  useEffect(() => {
    if (prefill.isError || isTokenInvalid(register.error)) {
      setToken(null)
      setExpired(true)
    }
  }, [prefill.isError, register.error])
  const locked = token !== null && prefill.isSuccess

  const submit = useOnce(() =>
    register.mutateAsync({
      phone: phone.trim(),
      firstName: firstName.trim(),
      lastName: lastName.trim(),
      houseId: Number(houseId),
      apartment: Number(apartment),
      statusId: Number(statusId),
      consent,
      locale: i18n.language,
      ...(locked ? { token: token ?? undefined } : {}),
    }),
  )

  const body = () => {
    if (options.isPending || (token !== null && prefill.isPending)) return <Loader />
    if (options.isError) return <Alert color="red">{t('errors.unknown')}</Alert>
    const { houses, statuses } = options.data
    if (houses.length === 0 || statuses.length === 0) return <Alert>{t('register.unavailable')}</Alert>
    if (register.isSuccess) return <Done botUrl={site.data?.botUrl ?? ''} />
    const house = houses.find((h) => String(h.id) === houseId)
    const complete = phone.trim() && firstName.trim() && lastName.trim() && houseId && apartment && statusId && consent
    const onSubmit = (e: FormEvent) => {
      e.preventDefault()
      if (complete) submit()
    }
    return (
      <form onSubmit={onSubmit}>
        <Stack maw={520}>
          <Text c="dimmed">{t('register.intro')}</Text>
          {expired && <LinkExpired botUrl={site.data?.botUrl ?? ''} />}
          <TextInput label={t('register.phone')} placeholder={t('login.phonePlaceholder')} type="tel" autoComplete="tel"
            value={phone} onChange={(e) => setPhone(e.currentTarget.value)} error={fields.phone}
            readOnly={locked} description={locked ? t('register.phoneConfirmed') : undefined} />
          <TextInput label={t('register.firstName')} autoComplete="given-name" value={firstName}
            onChange={(e) => setFirstName(e.currentTarget.value)} error={fields.firstName} />
          <TextInput label={t('register.lastName')} autoComplete="family-name" value={lastName}
            onChange={(e) => setLastName(e.currentTarget.value)} error={fields.lastName} />
          <Select label={t('register.house')} data={houses.map((h) => ({ value: String(h.id), label: h.name }))} value={houseId}
            onChange={(v) => {
              setHouseId(v)
              setApartment(null)
            }}
            error={fields.houseId} allowDeselect={false} />
          <Select label={t('register.apartment')} data={apartmentOptions(house)} value={apartment} onChange={setApartment}
            disabled={!house} searchable error={fields.apartment} allowDeselect={false} nothingFoundMessage={t('register.noApartment')} />
          <Select label={t('register.status')} data={statuses.map((s) => ({ value: String(s.id), label: text(s.names) || s.code }))}
            value={statusId} onChange={setStatusId} error={fields.statusId} allowDeselect={false} />
          <Checkbox label={t('register.consent')} checked={consent} onChange={(e) => setConsent(e.currentTarget.checked)} error={fields.consent} />
          {message && <Alert color="red">{message}</Alert>}
          <Button type="submit" loading={register.isPending} disabled={!complete}>
            {t('register.submit')}
          </Button>
        </Stack>
      </form>
    )
  }

  return <PageShell title={t('register.title')}>{body()}</PageShell>
}
