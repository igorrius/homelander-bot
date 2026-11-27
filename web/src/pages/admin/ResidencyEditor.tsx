import { Alert, Button, Group, Select } from '@mantine/core'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ResidencyInput } from '../../api/admin'
import type { House, ResidencyStatus } from '../../api/types'
import { apartmentOptions } from '../../lib/apartments'
import { useApiErrors } from '../../lib/errors'
import { useLocalized } from '../../lib/localized'

/** House, apartment (from that house's range) and residency status of one residency. */
export function ResidencyEditor({ initial, houses, statuses, busy, error, onSave, onCancel }: {
  initial?: ResidencyInput
  houses: House[]
  statuses: ResidencyStatus[]
  busy: boolean
  error: unknown
  onSave(v: ResidencyInput): void
  onCancel(): void
}) {
  const { t } = useTranslation()
  const text = useLocalized()
  const [houseId, setHouseId] = useState(initial ? String(initial.houseId) : null)
  const [apartment, setApartment] = useState(initial ? String(initial.apartment) : null)
  const [statusId, setStatusId] = useState(initial ? String(initial.statusId) : null)
  const { message, fields } = useApiErrors(error)
  const house = houses.find((h) => String(h.id) === houseId)
  const complete = houseId && apartment && statusId
  return (
    <Group align="flex-end" wrap="wrap">
      <Select label={t('register.house')} data={houses.map((h) => ({ value: String(h.id), label: h.name }))} value={houseId}
        onChange={(v) => {
          setHouseId(v)
          setApartment(null)
        }}
        error={fields.houseId} allowDeselect={false} w={180} />
      <Select label={t('register.apartment')} data={apartmentOptions(house)} value={apartment} onChange={setApartment} disabled={!house}
        searchable error={fields.apartment} allowDeselect={false} w={130} />
      <Select label={t('register.status')} data={statuses.map((s) => ({ value: String(s.id), label: text(s.names) || s.code }))}
        value={statusId} onChange={setStatusId} error={fields.statusId} allowDeselect={false} w={180} />
      <Button loading={busy} disabled={!complete}
        onClick={() => onSave({ houseId: Number(houseId), apartment: Number(apartment), statusId: Number(statusId) })}>
        {t('admin.user.saveResidency')}
      </Button>
      <Button variant="default" onClick={onCancel}>
        {t('common.cancel')}
      </Button>
      {message && <Alert color="red" w="100%">{message}</Alert>}
    </Group>
  )
}
