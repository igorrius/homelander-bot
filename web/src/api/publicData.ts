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
