import { useTranslation } from 'react-i18next'

/** Translated audit action; an action without a text is shown as it is. */
export function useAuditAction() {
  const { t } = useTranslation()
  return (action: string) => t(`audit.action.${action}`, { defaultValue: action })
}
