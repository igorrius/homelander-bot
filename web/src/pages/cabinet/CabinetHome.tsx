import { Stack, Text, Title } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useMe } from '../../api/public'
import { cabinetHomeCards } from '../../cabinet/homeCards'

/** The cabinet's index page: a greeting, then the registered home cards in order. */
export function CabinetHome() {
  const { t } = useTranslation()
  const name = useMe().data?.firstName.trim()
  return (
    <Stack gap="lg">
      <Stack gap={2}>
        <Title order={1} size="h2">
          {name ? t('cabinet.home.greeting', { name }) : t('cabinet.home.greetingNoName')}
        </Title>
        <Text c="dimmed">{t('cabinet.home.subtitle')}</Text>
      </Stack>
      {cabinetHomeCards.map(({ key, Component }) => (
        <Component key={key} />
      ))}
    </Stack>
  )
}
