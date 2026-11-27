import type { ReactElement } from 'react'
import { QueryClient } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useLocation } from 'react-router'
import App from '../App'
import { Providers } from '../Providers'

function LocationProbe() {
  const { pathname, search, hash } = useLocation()
  return <div data-testid="location" hidden>{pathname + search + hash}</div>
}

/** Renders the whole app at path with fresh query state and no retries. */
export function renderApp(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const user = userEvent.setup()
  const result = render(
    <Providers client={client} env="test">
      <MemoryRouter initialEntries={[path]}>
        <App />
        <LocationProbe />
      </MemoryRouter>
    </Providers>,
  )
  return { ...result, user, client }
}

/** Current router path, query and hash, e.g. "/admin/users?page=2". */
export function currentLocation(): string {
  return screen.getByTestId('location').textContent ?? ''
}

/** Renders one component inside the app providers (theme, i18n, a fresh query client). */
export function renderWithProviders(ui: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <Providers client={client} env="test">
      <MemoryRouter>{ui}</MemoryRouter>
    </Providers>,
  )
}
