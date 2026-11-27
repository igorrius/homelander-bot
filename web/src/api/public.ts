import { useQuery } from '@tanstack/react-query'
import { api } from './client'
import type { Site } from './types'

/** Query keys of the public API. */
export const qk = { site: ['site'], me: ['me'], registerOptions: ['registerOptions'] } as const

export function useSite() {
  return useQuery({ queryKey: qk.site, queryFn: () => api<Site>('/site'), staleTime: 60_000 })
}
