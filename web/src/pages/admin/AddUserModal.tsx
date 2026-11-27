import { Alert, Button, Checkbox, Group, Modal, Stack, TextInput } from '@mantine/core'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useAddUser } from '../../api/admin'
import { useApiErrors } from '../../lib/errors'
import { useOnce } from '../../lib/useOnce'

/** Adds a user directly (no application); the card opens on success. */
export function AddUserModal({ opened, onClose }: { opened: boolean; onClose(): void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const add = useAddUser()
  const [phone, setPhone] = useState('')
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [isResident, setResident] = useState(false)
  const [isAdmin, setAdmin] = useState(false)
  const { message, fields } = useApiErrors(add.error)
  const save = useOnce(async () => {
    const card = await add.mutateAsync({ phone: phone.trim(), firstName: firstName.trim(), lastName: lastName.trim(), isResident, isAdmin })
    navigate(`/admin/users/${card.user.id}`)
  })
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    save()
  }
  return (
    <Modal opened={opened} onClose={onClose} title={t('admin.users.add')} centered>
      <form onSubmit={onSubmit}>
        <Stack>
          <TextInput label={t('register.phone')} placeholder={t('login.phonePlaceholder')} type="tel" value={phone}
            onChange={(e) => setPhone(e.currentTarget.value)} error={fields.phone} data-autofocus />
          <TextInput label={t('register.firstName')} value={firstName} onChange={(e) => setFirstName(e.currentTarget.value)} error={fields.firstName} />
          <TextInput label={t('register.lastName')} value={lastName} onChange={(e) => setLastName(e.currentTarget.value)} error={fields.lastName} />
          <Checkbox label={t('admin.user.isResident')} checked={isResident} onChange={(e) => setResident(e.currentTarget.checked)} />
          <Checkbox label={t('admin.user.isAdmin')} checked={isAdmin} onChange={(e) => setAdmin(e.currentTarget.checked)} />
          {message && <Alert color="red">{message}</Alert>}
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" loading={add.isPending}>
              {t('common.save')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}
