import { Button, Card, Group, Stack, Text, ThemeIcon } from '@mantine/core'
import { IconDeviceMobileDown, IconShare2 } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useInstall } from '../../lib/pwa'

/** Suggests installing the cabinet as an app; hidden when it cannot be, already is, or was dismissed. */
export function InstallCard() {
  const { t } = useTranslation()
  const { mode, install, dismiss } = useInstall()
  if (!mode) return null
  const Icon = mode === 'ios' ? IconShare2 : IconDeviceMobileDown
  return (
    <Card bg="var(--mantine-primary-color-light)" style={{ borderColor: 'transparent' }}>
      <Group wrap="nowrap" align="flex-start" gap="md">
        <ThemeIcon size={44} radius="md" variant="filled">
          <Icon size={24} stroke={1.7} />
        </ThemeIcon>
        <Stack gap={4} style={{ flex: 1, minWidth: 0 }}>
          <Text fw={650}>{t('cabinet.install.title')}</Text>
          <Text size="sm">{mode === 'ios' ? t('cabinet.install.iosText') : t('cabinet.install.text')}</Text>
          <Group gap="xs" mt={8}>
            {mode === 'prompt' && (
              <Button size="xs" onClick={() => void install()}>
                {t('cabinet.install.button')}
              </Button>
            )}
            <Button size="xs" variant="subtle" color="gray" onClick={dismiss}>
              {t('cabinet.install.later')}
            </Button>
          </Group>
        </Stack>
      </Group>
    </Card>
  )
}
