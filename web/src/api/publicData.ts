import { useQuery } from '@tanstack/react-query'
import { api } from './client'
import type { DataColumn, DataType } from './types'

/** One public item as GET /api/v1/public/data returns it (image values are asset URLs). */
export type PublicItem =
  | { kind: 'value'; type: DataType; value: unknown }
  | { kind: 'table'; columns: DataColumn[]; rows: Record<string, unknown>[] }

/** Every public item by key; loaded only when enabled (the built-in page has cards to show). */
export function usePublicData(enabled = true) {
  return useQuery({
    queryKey: ['publicData'],
    queryFn: () => api<Record<string, PublicItem>>('/public/data'),
    staleTime: 60_000,
    enabled,
  })
}

/** Query key of /me/data; dropped on logout. */
export const myDataKey = ['meData'] as const

/**
 * What the signed-in user may read on the site (GET /api/v1/me/data): public items and, for a resident or an
 * admin, residents items, in the shape of /public/data. Images of residents items are signed URLs that expire
 * in 1–2 hours, so the query refreshes every half hour while a page is open.
 */
export function useMyData(userId: number | undefined) {
  return useQuery({
    queryKey: [...myDataKey, userId],
    queryFn: () => api<Record<string, PublicItem>>('/me/data'),
    enabled: userId !== undefined,
    staleTime: 60_000,
    refetchInterval: 30 * 60_000,
  })
}
