import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { i18n } from './i18n'
import { theme } from './theme'

/** App-wide providers. Tests pass their own query client and env="test" (no transitions or portals). */
export function Providers({ children, client, env }: { children: ReactNode; client?: QueryClient; env?: 'test' }) {
  const [queryClient] = useState(() => client ?? new QueryClient({ defaultOptions: { queries: { retry: 1 } } }))
  return (
    <I18nextProvider i18n={i18n}>
      <MantineProvider theme={theme} defaultColorScheme="auto" env={env}>
        <Notifications />
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </MantineProvider>
    </I18nextProvider>
  )
}
