import { Alert, Stack, Switch } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useSetModuleEnabled } from '../../api/modules'
import type { AdminModule } from '../../api/types'
import { errorText } from '../../lib/errors'

/** The on/off switch of one module, with the error of the last change. */
export function ModuleSwitch({ module: m }: { module: AdminModule }) {
  const { t } = useTranslation()
  const set = useSetModuleEnabled()
  return (
    <Stack gap="xs">
      <Switch
        checked={m.enabled}
        disabled={set.isPending}
        onChange={(e) => set.mutate({ code: m.code, enabled: e.currentTarget.checked })}
        label={m.enabled ? t('admin.modules.on') : t('admin.modules.off')}
        aria-label={t(m.nameKey)}
        size="md"
      />
      {set.error && <Alert color="red" p="xs">{errorText(t, set.error)}</Alert>}
    </Stack>
  )
}
