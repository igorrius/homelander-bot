import { Alert, Button, Group, NumberInput, SimpleGrid, Stack, Switch, TextInput } from '@mantine/core'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import type { HouseInput } from '../../api/admin'
import type { House } from '../../api/types'
import { useApiErrors } from '../../lib/errors'

type Num = number | string
const optional = (v: Num): number | null => (v === '' || v === null ? null : Number(v))
const required = (v: Num): number => (v === '' ? 0 : Number(v))

/** Settings of one house; empty optional numbers are sent as null. */
export function HouseForm({ initial, busy, error, onSave }: { initial?: House; busy: boolean; error: unknown; onSave(v: HouseInput): void }) {
  const { t } = useTranslation()
  const [name, setName] = useState(initial?.name ?? '')
  const [address, setAddress] = useState(initial?.address ?? '')
  const [latitude, setLatitude] = useState<Num>(initial?.latitude ?? '')
  const [longitude, setLongitude] = useState<Num>(initial?.longitude ?? '')
  const [floors, setFloors] = useState<Num>(initial?.floors ?? '')
  const [from, setFrom] = useState<Num>(initial?.apartmentFrom ?? '')
  const [to, setTo] = useState<Num>(initial?.apartmentTo ?? '')
  const [sortOrder, setSortOrder] = useState<Num>(initial?.sortOrder ?? 0)
  const [archived, setArchived] = useState(initial?.archived ?? false)
  const { message, fields } = useApiErrors(error)
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    onSave({
      name: name.trim(),
      address: address.trim(),
      latitude: optional(latitude),
      longitude: optional(longitude),
      floors: optional(floors),
      apartmentFrom: required(from),
      apartmentTo: required(to),
      sortOrder: required(sortOrder),
      archived,
    })
  }
  return (
    <form onSubmit={onSubmit}>
      <Stack>
        <SimpleGrid cols={{ base: 1, sm: 2 }}>
          <TextInput label={t('admin.house.name')} description={t('admin.house.nameHint')} value={name} onChange={(e) => setName(e.currentTarget.value)} error={fields.name} />
          <TextInput label={t('admin.house.address')} value={address} onChange={(e) => setAddress(e.currentTarget.value)} error={fields.address} />
          <NumberInput label={t('admin.house.apartmentFrom')} value={from} onChange={setFrom} min={1} allowDecimal={false} error={fields.apartmentFrom} />
          <NumberInput label={t('admin.house.apartmentTo')} value={to} onChange={setTo} min={1} allowDecimal={false} error={fields.apartmentTo} />
          <NumberInput label={t('admin.house.floors')} value={floors} onChange={setFloors} min={1} allowDecimal={false} error={fields.floors} />
          <NumberInput label={t('admin.house.sortOrder')} value={sortOrder} onChange={setSortOrder} allowDecimal={false} error={fields.sortOrder} />
          <NumberInput label={t('admin.house.latitude')} value={latitude} onChange={setLatitude} decimalScale={6} min={-90} max={90} error={fields.latitude} />
          <NumberInput label={t('admin.house.longitude')} value={longitude} onChange={setLongitude} decimalScale={6} min={-180} max={180} error={fields.longitude} />
        </SimpleGrid>
        {initial && <Switch label={t('admin.houses.archived')} description={t('admin.house.archivedHint')} checked={archived} onChange={(e) => setArchived(e.currentTarget.checked)} />}
        {message && <Alert color="red">{message}</Alert>}
        <Group>
          <Button type="submit" loading={busy}>
            {t('common.save')}
          </Button>
        </Group>
      </Stack>
    </form>
  )
}
