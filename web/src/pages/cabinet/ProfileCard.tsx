import { Badge, Card, Divider, Group, Stack, Text, Title } from '@mantine/core'
import { IconInfoCircle, IconUserCircle } from '@tabler/icons-react'
import { Fragment, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useMyHome } from '../../api/cabinet'
import { useMe } from '../../api/public'
import { fullName, maskPhone } from '../../lib/format'

/** The signed-in user's own details, read-only: admins change them. */
export function ProfileCard() {
  const { t } = useTranslation()
  const me = useMe().data
  const home = useMyHome()
  if (!me) return null
  const linked = home.data?.telegramLinked
  const rows: { label: string; value: ReactNode }[] = [
    { label: t('cabinet.profile.name'), value: fullName(me) },
    { label: t('cabinet.profile.phone'), value: <span style={{ fontVariantNumeric: 'tabular-nums' }}>{maskPhone(me.phone)}</span> },
    {
      label: t('cabinet.profile.telegram'),
      value:
        linked === undefined ? null : (
          <Badge variant="light" color={linked ? 'teal' : 'gray'} radius="sm">
            {linked ? t('cabinet.profile.linked') : t('cabinet.profile.notLinked')}
          </Badge>
        ),
    },
    { label: t('cabinet.profile.language'), value: me.locale ? t(`language.${me.locale}`, { defaultValue: me.locale }) : '—' },
  ]
  return (
    <Card data-testid="cabinet-profile">
      <Group gap="xs" mb="sm">
        <IconUserCircle size={20} stroke={1.7} color="var(--mantine-color-teal-filled)" />
        <Title order={2} size="h4">
          {t('cabinet.profile.title')}
        </Title>
      </Group>
      <Stack gap={0}>
        {rows.map((r, i) => (
          <Fragment key={r.label}>
            {i > 0 && <Divider />}
            <Group justify="space-between" wrap="nowrap" gap="md" py="sm">
              <Text size="sm" c="dimmed">
                {r.label}
              </Text>
              <Text component="div" size="sm" fw={500} ta="right" style={{ minWidth: 0, overflowWrap: 'anywhere' }}>
                {r.value}
              </Text>
            </Group>
          </Fragment>
        ))}
      </Stack>
      <Group gap={6} wrap="nowrap" mt="xs" align="flex-start">
        <IconInfoCircle size={16} color="var(--mantine-color-dimmed)" style={{ flex: 'none', marginTop: 2 }} />
        <Text size="xs" c="dimmed">
          {t('cabinet.profile.hint')}
        </Text>
      </Group>
    </Card>
  )
}
