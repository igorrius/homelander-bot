import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { AdminModule, CabinetModule } from './types'

/** Query keys of modules: the signed-in user's list lives under 'cabinet' so logout drops it. */
export const qkm = {
  mine: ['cabinet', 'modules'] as const,
  admin: ['admin', 'modules'] as const,
}

/** Modules that are turned on, with their cabinet navigation entries. */
export function useMyModules() {
  return useQuery({ queryKey: qkm.mine, queryFn: () => api<CabinetModule[]>('/modules') })
}

/** Every built-in module with its state, for admins. */
export function useAdminModules() {
  return useQuery({ queryKey: qkm.admin, queryFn: () => api<AdminModule[]>('/admin/modules') })
}

/** Whether the module with this code is on; undefined while the list loads or failed. */
export function useModuleEnabled(code: string): boolean | undefined {
  const q = useAdminModules()
  return q.data?.find((m) => m.code === code)?.enabled
}

/** Turns a module on or off. */
export function useSetModuleEnabled() {
  const qc = useQueryClient()
  return useMutation<AdminModule, ApiError, { code: string; enabled: boolean }>({
    mutationFn: ({ code, enabled }) => api<AdminModule>(`/admin/modules/${code}`, { method: 'PUT', body: JSON.stringify({ enabled }) }),
    onSuccess: (m) => {
      qc.setQueryData<AdminModule[]>(qkm.admin, (list) => list?.map((x) => (x.code === m.code ? m : x)))
      void qc.invalidateQueries({ queryKey: qkm.admin })
      void qc.invalidateQueries({ queryKey: qkm.mine })
      // A module's pages and its owned data items change with it.
      void qc.invalidateQueries({ queryKey: ['admin', m.code] })
      void qc.invalidateQueries({ queryKey: ['admin', 'data'] })
    },
  })
}
