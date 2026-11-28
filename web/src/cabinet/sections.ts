import { IconHome, IconNews, IconPuzzle, type Icon } from '@tabler/icons-react'
import { useMyModules } from '../api/modules'

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

/** Icons the API names for module sections; an unknown name gets the generic module icon. */
const moduleIcons: Record<string, Icon> = {
  news: IconNews,
}

/**
 * The cabinet navigation for the signed-in user: the base sections, then the
 * sections of the modules an admin turned on (GET /api/v1/modules). While that
 * list loads or fails, only the base sections.
 */
export function useCabinetSections(): readonly CabinetSection[] {
  const modules = useMyModules()
  if (!modules.data) return baseSections
  return [
    ...baseSections,
    ...modules.data.flatMap((m) =>
      // Only cabinet pages belong in the cabinet navigation.
      m.nav.filter((n) => n.path.startsWith('/my/')).map((n) => ({ key: `${m.code}.${n.key}`, to: n.path, icon: moduleIcons[n.icon] ?? IconPuzzle, labelKey: n.labelKey })),
    ),
  ]
}

/** Whether a section is the current page. */
export function isSectionActive(s: CabinetSection, pathname: string): boolean {
  return s.exact ? pathname === s.to || pathname === s.to + '/' : pathname === s.to || pathname.startsWith(s.to + '/')
}
