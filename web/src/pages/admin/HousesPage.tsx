import { Alert, Anchor, Badge, Button, Loader, Modal, Stack, Switch, Table, Text } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { useState } from 'react'
import { IconBuildingCommunity, IconPlus } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { useCreateHouse, useHouses } from '../../api/admin'
import { errorText } from '../../lib/errors'
import { HouseForm } from './HouseForm'
import { PageHeader } from '../../components/PageHeader'

export function HousesPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [showArchived, setShowArchived] = useState(false)
  const houses = useHouses(showArchived)
  const create = useCreateHouse()
  const [adding, { open, close }] = useDisclosure(false)
  return (
    <Stack>
      <PageHeader icon={IconBuildingCommunity} title={t('admin.houses.title')}
        actions={
          <Button leftSection={<IconPlus size={16} />} onClick={() => {
            create.reset()
            open()
          }}>
            {t('admin.houses.add')}
          </Button>
        } />
      <Switch label={t('admin.houses.showArchived')} checked={showArchived} onChange={(e) => setShowArchived(e.currentTarget.checked)} />
      {houses.isPending ? (
        <Loader />
      ) : houses.isError ? (
        <Alert color="red">{errorText(t, houses.error)}</Alert>
      ) : houses.data.length === 0 ? (
        <Text>{t('admin.houses.empty')}</Text>
      ) : (
        <Table striped>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('admin.house.name')}</Table.Th>
              <Table.Th>{t('admin.house.address')}</Table.Th>
              <Table.Th>{t('admin.houses.apartments')}</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {houses.data.map((h) => (
              <Table.Tr key={h.id}>
                <Table.Td>
                  <Anchor component={Link} to={`/admin/houses/${h.id}`}>
                    {h.name}
                  </Anchor>
                </Table.Td>
                <Table.Td>{h.address}</Table.Td>
                <Table.Td>{`${h.apartmentFrom}–${h.apartmentTo}`}</Table.Td>
                <Table.Td>{h.archived && <Badge color="gray">{t('admin.houses.archived')}</Badge>}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
      <Modal opened={adding} onClose={close} title={t('admin.houses.add')} size="lg" centered>
        <HouseForm busy={create.isPending} error={create.error}
          onSave={(v) => create.mutate(v, { onSuccess: (h) => navigate(`/admin/houses/${h.id}`) })} />
      </Modal>
    </Stack>
  )
}
