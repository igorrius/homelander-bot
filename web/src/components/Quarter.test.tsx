import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { renderWithProviders } from '../test/render'
import { Quarter } from './Quarter'

const houses = (n: number, floors: number | null = null) =>
  Array.from({ length: n }, (_, i) => ({ id: i + 1, name: `House ${i + 1}`, floors: floors ?? 3 + ((i * 7) % 23) }))

function attrsHaveNoNaN(root: Element) {
  for (const el of root.querySelectorAll('*')) {
    for (const a of el.attributes) expect(a.value, `${el.tagName} ${a.name}`).not.toMatch(/NaN|undefined|Infinity/)
  }
}

describe('Quarter', () => {
  it('draws 24 houses without NaN and names them for screen readers', () => {
    const { container } = renderWithProviders(<Quarter houses={houses(24)} />)
    const svg = screen.getByRole('img')
    expect(within(container).getAllByTestId('house')).toHaveLength(24)
    expect(svg.getAttribute('aria-label')).toContain('House 1')
    expect(svg.getAttribute('aria-label')).toContain('House 24')
    attrsHaveNoNaN(container)
  })

  it('draws a house without floors and truncates a long name with the full name as a tooltip', () => {
    const long = 'Будинок на дуже довгій тестовій вулиці'
    const { container } = renderWithProviders(<Quarter houses={[{ id: 1, name: long, floors: null }, { id: 2, name: 'B', floors: 40 }]} />)
    const [first] = within(container).getAllByTestId('house')
    expect(first.querySelector('title')?.textContent).toBe(long)
    expect(within(container).getByText(`${long.slice(0, 17)}…`)).toBeInTheDocument()
    attrsHaveNoNaN(container)
  })

  it('draws an empty yard when there are no houses', () => {
    const { container } = renderWithProviders(<Quarter houses={[]} />)
    expect(within(container).queryAllByTestId('house')).toHaveLength(0)
    expect(within(container).getAllByTestId('tree').length).toBeGreaterThan(2)
    attrsHaveNoNaN(container)
  })
})
