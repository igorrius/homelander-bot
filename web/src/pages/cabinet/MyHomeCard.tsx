import { Alert, Badge, Card, Group, Paper, SimpleGrid, Skeleton, Stack, Text, ThemeIcon, Title } from '@mantine/core'
import { IconBuilding, IconHome } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useMyHome } from '../../api/cabinet'
import type { Home } from '../../api/types'
import { errorText } from '../../lib/errors'
import { useLocalized } from '../../lib/localized'

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Stack gap={2}>
      <Text size="xs" c="dimmed" tt="uppercase" fw={600}>
        {label}
      </Text>
      <Text component="div" fw={600}>
        {children}
      </Text>
    </Stack>
  )
}

function HomeBlock({ home }: { home: Home }) {
  const { t } = useTranslation()
  const text = useLocalized()
  const status = text(home.statusNames)
  return (
    <Paper withBorder p="md" data-testid={`cabinet-home-${home.id}`}>
      <Group gap="sm" wrap="nowrap" align="flex-start">
        <ThemeIcon variant="light" size={40} radius="md">
          <IconBuilding size={22} stroke={1.6} />
        </ThemeIcon>
        <Stack gap={0} style={{ minWidth: 0 }}>
          <Text fw={650}>{home.houseName}</Text>
          {home.address && (
            <Text size="sm" c="dimmed">
              {home.address}
            </Text>
          )}
        </Stack>
      </Group>
      <SimpleGrid cols={3} spacing="sm" mt="md">
        <Fact label={t('cabinet.myHome.apartment')}>{home.apartment}</Fact>
        {home.floors != null && <Fact label={t('cabinet.myHome.floors')}>{home.floors}</Fact>}
        {status && (
          <Fact label={t('cabinet.myHome.status')}>
            <Badge variant="light" size="md" radius="sm">
              {status}
            </Badge>
          </Fact>
        )}
      </SimpleGrid>
    </Paper>
  )
}

/** "My home": every apartment of the signed-in user, with its house. */
export function MyHomeCard() {
  const { t } = useTranslation()
  const home = useMyHome()
  return (
    <Card>
      <Group gap="xs" mb="md">
        <IconHome size={20} stroke={1.7} color="var(--mantine-color-teal-filled)" />
        <Title order={2} size="h4">
          {t('cabinet.myHome.title')}
        </Title>
      </Group>
      {home.isPending ? (
        <Skeleton h={110} />
      ) : home.error ? (
        <Alert color="red">{errorText(t, home.error)}</Alert>
      ) : home.data.homes.length === 0 ? (
        <Stack gap={4}>
          <Text fw={500}>{t('cabinet.myHome.empty')}</Text>
          <Text size="sm" c="dimmed">
            {t('cabinet.myHome.emptyHint')}
          </Text>
        </Stack>
      ) : (
        <Stack gap="sm">
          {home.data.homes.map((h) => (
            <HomeBlock key={h.id} home={h} />
          ))}
        </Stack>
      )}
    </Card>
  )
}
