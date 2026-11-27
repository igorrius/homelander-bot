import { Anchor } from '@mantine/core'
import { IconArrowLeft } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

/** A small "back" link with an arrow icon (never a text glyph). */
export function BackLink({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Anchor component={Link} to={to} size="sm" style={{ display: 'inline-flex', alignItems: 'center', gap: 4, alignSelf: 'flex-start' }}>
      <IconArrowLeft size={15} stroke={1.8} aria-hidden />
      {children}
    </Anchor>
  )
}
