import { QueryClient } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useLocation } from 'react-router'
import App from '../App'
import { Providers } from '../Providers'

function LocationProbe() {
  const { pathname, search } = useLocation()
  return <div data-testid="location" hidden>{pathname + search}</div>
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

/** Current router path and query, e.g. "/admin/users?page=2". */
export function currentLocation(): string {
  return screen.getByTestId('location').textContent ?? ''
}
