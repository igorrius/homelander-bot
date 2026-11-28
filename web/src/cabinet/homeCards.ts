import type { ComponentType } from 'react'
import { FeedCard } from '../pages/cabinet/FeedCard'
import { InstallCard } from '../pages/cabinet/InstallCard'
import { MyHomeCard } from '../pages/cabinet/MyHomeCard'
import { ProfileCard } from '../pages/cabinet/ProfileCard'

/** One card of the cabinet home page. A card that has nothing to show renders null. */
export interface HomeCard {
  key: string
  Component: ComponentType
}

/**
 * The cabinet home, top to bottom. Modules add their card here (the feed's
 * latest posts go first) without changing the page.
 */
export const cabinetHomeCards: readonly HomeCard[] = [
  { key: 'feed', Component: FeedCard },
  { key: 'install', Component: InstallCard },
  { key: 'myHome', Component: MyHomeCard },
  { key: 'profile', Component: ProfileCard },
]
