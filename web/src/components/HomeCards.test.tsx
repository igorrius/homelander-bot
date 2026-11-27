import { screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { PublicItem } from '../api/publicData'
import { i18n } from '../i18n'
import { renderWithProviders } from '../test/render'
import { HomeCards } from './HomeCards'

// Synthetic data only.
const data: Record<string, PublicItem> = {
  'office.phone': { kind: 'value', type: 'phone', value: '+380 44 000 00 00' },
  'house.address': { kind: 'value', type: 'localized_text', value: { uk: 'вул. Тестова, 1', en: '1 Test St' } },
  'site.link': { kind: 'value', type: 'url', value: 'https://example.test/info' },
  'has.parking': { kind: 'value', type: 'bool', value: true },
  contacts: {
    kind: 'table',
    columns: [{ name: 'name', label: 'Name', type: 'text' }, { name: 'phone', label: 'Phone', type: 'phone' }, { name: 'web', label: 'Web', type: 'url' }],
    rows: [{ id: 1, name: 'Plumber', phone: '+380 44 000 00 01', web: 'https://plumber.test' }, { id: 2, name: 'Guard', phone: null, web: null }],
  },
}

const cards = () => within(screen.getByRole('list', { name: /./ })).getAllByRole('listitem')

describe('HomeCards', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('shows the chosen items in order with their titles', () => {
    renderWithProviders(<HomeCards items={[{ key: 'house.address', title: {} }, { key: 'office.phone', title: { uk: 'Офіс', en: 'Office' } }]} data={data} />)
    const [first, second] = cards()
    expect(first).toHaveTextContent('House address')
    expect(first).toHaveTextContent('вул. Тестова, 1')
    expect(second).toHaveTextContent('Офіс')
  })

  it('skips a key that is missing or no longer public', () => {
    renderWithProviders(<HomeCards items={[{ key: 'gone', title: { uk: 'Зникло' } }, { key: 'has.parking', title: {} }]} data={data} />)
    expect(cards()).toHaveLength(1)
    expect(screen.queryByText('Зникло')).not.toBeInTheDocument()
    expect(cards()[0]).toHaveTextContent('Так')
  })

  it('links a phone and a url', () => {
    renderWithProviders(<HomeCards items={[{ key: 'office.phone', title: {} }, { key: 'site.link', title: {} }]} data={data} />)
    expect(screen.getByRole('link', { name: /\+380 44 000 00 00/ })).toHaveAttribute('href', 'tel:+380440000000')
    const link = screen.getByRole('link', { name: /example\.test/ })
    expect(link).toHaveAttribute('href', 'https://example.test/info')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link.getAttribute('rel')).toContain('noopener')
  })

  it('lists table rows with linked cells', () => {
    renderWithProviders(<HomeCards items={[{ key: 'contacts', title: {} }]} data={data} />)
    const card = cards()[0]
    expect(card).toHaveTextContent('Contacts')
    expect(card).toHaveTextContent('Plumber')
    expect(card).toHaveTextContent('Guard')
    expect(within(card).getByRole('link', { name: /\+380 44 000 00 01/ })).toHaveAttribute('href', 'tel:+380440000001')
    expect(within(card).getByRole('link', { name: /plumber\.test/ })).toHaveAttribute('href', 'https://plumber.test')
  })

  it('renders nothing without items', () => {
    const { container } = renderWithProviders(<HomeCards items={[]} data={data} />)
    expect(container.querySelector('ul')).toBeNull()
  })
})
