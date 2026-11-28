import { Badge, Input, SegmentedControl } from '@mantine/core'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { visibilities, type Visibility } from '../../api/types'

const colors: Record<Visibility, string> = { public: 'green', residents: 'blue', admins: 'gray' }

/** Who sees a data item: everyone, residents or admins, with the help text about site pages. */
export function VisibilityControl({ value, onChange, disabled }: { value: Visibility; onChange(v: Visibility): void; disabled?: boolean }) {
  const { t } = useTranslation()
  const id = useId()
  return (
    <Input.Wrapper label={t('admin.data.access')} description={t('admin.data.visibilityHint')} labelProps={{ id }}>
      <SegmentedControl mt={4} aria-labelledby={id} value={value} disabled={disabled} onChange={(v) => onChange(v as Visibility)}
        data={visibilities.map((v) => ({ value: v, label: t(`admin.data.visibility.${v}`) }))} />
    </Input.Wrapper>
  )
}

/** The visibility of an item as a coloured badge. */
export function VisibilityBadge({ value }: { value: Visibility }) {
  const { t } = useTranslation()
  return <Badge variant="light" color={colors[value]}>{t(`admin.data.visibility.${value}`)}</Badge>
}
