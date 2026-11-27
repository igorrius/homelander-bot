import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { ApiError } from '../api/client'

/** Translated text of an API (or network) error; unknown codes get the generic text. */
export function errorText(t: TFunction, err: unknown): string {
  if (err instanceof ApiError) {
    const key = `errors.${err.code}`
    if (t(key, { defaultValue: '' })) return t(key)
  }
  return t('errors.unknown')
}

/** Field name → translated reason for a validation error, else {}. */
export function fieldErrors(t: TFunction, err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) return {}
  const out: Record<string, string> = {}
  for (const [field, reason] of Object.entries(err.fields)) {
    const key = `validation.${reason}`
    out[field] = t(key, { defaultValue: '' }) ? t(key) : t('validation.invalid')
  }
  return out
}

/**
 * Message and field errors for err. A validation error with field details
 * has no top-level message: the fields say it.
 */
export function useApiErrors(err: unknown): { message: string | null; fields: Record<string, string> } {
  const { t } = useTranslation()
  if (!err) return { message: null, fields: {} }
  const fields = fieldErrors(t, err)
  return { message: Object.keys(fields).length > 0 ? null : errorText(t, err), fields }
}
