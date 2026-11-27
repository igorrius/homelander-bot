import { useMutation, useQuery } from '@tanstack/react-query'
import { api, ApiError } from './client'

/** What the consent page shows about an agent asking for access. */
export interface AuthorizeInfo {
  clientName: string
  redirectHost: string
  signedIn: boolean
  allowed: boolean
  /** Why the signed-in user may not allow it. */
  reason?: 'not_admin' | 'mcp_off'
}

export interface AuthorizeRequest {
  clientId: string
  redirectUri: string
  codeChallenge: string
  codeChallengeMethod: string
  state: string
  resource: string
  approve: boolean
}

export function useAuthorizeInfo(clientId: string, redirectUri: string, signedIn: boolean) {
  return useQuery<AuthorizeInfo, ApiError>({
    queryKey: ['oauth', 'info', clientId, redirectUri, signedIn],
    queryFn: () =>
      api<AuthorizeInfo>(`/oauth/authorize-info?${new URLSearchParams({ client_id: clientId, redirect_uri: redirectUri })}`),
    enabled: clientId !== '' && redirectUri !== '',
    retry: false,
  })
}

export function useAuthorize() {
  return useMutation<{ redirect: string }, ApiError, AuthorizeRequest>({
    mutationFn: (body) => api<{ redirect: string }>('/oauth/authorize', { method: 'POST', body: JSON.stringify(body) }),
  })
}
