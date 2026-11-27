import { Alert, Button, Checkbox, Loader, Select, Stack, Text, TextInput, Title } from '@mantine/core'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useRegister, useRegisterOptions, useSite } from '../api/public'
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

export function RegisterPage() {
  const { t, i18n } = useTranslation()
  const options = useRegisterOptions()
  const site = useSite()
  const register = useRegister()
  const text = useLocalized()
  const [phone, setPhone] = useState('')
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [houseId, setHouseId] = useState<string | null>(null)
  const [apartment, setApartment] = useState<string | null>(null)
  const [statusId, setStatusId] = useState<string | null>(null)
  const [consent, setConsent] = useState(false)
  const { message, fields } = useApiErrors(register.error)

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
    }),
  )

  const body = () => {
    if (options.isPending) return <Loader />
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
          <TextInput label={t('register.phone')} placeholder={t('login.phonePlaceholder')} type="tel" autoComplete="tel"
            value={phone} onChange={(e) => setPhone(e.currentTarget.value)} error={fields.phone} />
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
