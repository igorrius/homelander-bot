import { IconHome, type Icon } from '@tabler/icons-react'

/** One entry of the cabinet navigation (sidebar on wide screens, bottom bar on phones). */
export interface CabinetSection {
  key: string
  to: string
  icon: Icon
  /** Translation key of the label. */
  labelKey: string
  /** Active only on this exact path, not on the paths below it. */
  exact?: boolean
}

/** Sections every cabinet has, in order. */
export const baseSections: readonly CabinetSection[] = [
  { key: 'home', to: '/my', icon: IconHome, labelKey: 'cabinet.nav.home', exact: true },
]

/**
 * The cabinet navigation for the signed-in user. Module sections (the feed and
 * later modules) are appended here for the modules an admin turned on, from
 * GET /api/v1/modules; the layout only renders this list.
 */
export function useCabinetSections(): readonly CabinetSection[] {
  return baseSections
}

/** Whether a section is the current page. */
export function isSectionActive(s: CabinetSection, pathname: string): boolean {
  return s.exact ? pathname === s.to || pathname === s.to + '/' : pathname === s.to || pathname.startsWith(s.to + '/')
}
