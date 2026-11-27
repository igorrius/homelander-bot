import { Stack, Textarea, TextInput } from '@mantine/core'
import type { LocalizedText } from '../api/types'
import { supportedLocales } from '../i18n'

const tags = { uk: 'УК', en: 'EN' } as const

/** One input per supported locale for admin-entered text. */
export function LocalizedTextInputs({ label, value, onChange, error, multiline = false }: {
  label: string
  value: LocalizedText
  onChange(v: LocalizedText): void
  error?: string
  multiline?: boolean
}) {
  return (
    <Stack gap="xs">
      {supportedLocales.map((loc, i) => {
        const props = {
          label: `${label} (${tags[loc]})`,
          value: value[loc] ?? '',
          error: i === supportedLocales.length - 1 ? error : undefined,
          onChange: (e: { currentTarget: { value: string } }) => onChange({ ...value, [loc]: e.currentTarget.value }),
        }
        return multiline ? <Textarea key={loc} autosize minRows={3} {...props} /> : <TextInput key={loc} {...props} />
      })}
    </Stack>
  )
}
