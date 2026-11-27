import { render, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { HousesIllustration } from './HousesIllustration'

describe('HousesIllustration', () => {
  it('draws one unlabeled house when there are none', () => {
    const { container } = render(<HousesIllustration houses={[]} />)
    expect(within(container).getAllByTestId('house')).toHaveLength(1)
    expect(container.querySelectorAll('text')).toHaveLength(0)
  })

  it('draws one labelled house per house', () => {
    const houses = ['A', 'B', 'C', 'D', 'E'].map((n, i) => ({ id: i + 1, name: `House ${n}` }))
    const { container } = render(<HousesIllustration houses={houses} />)
    expect(within(container).getAllByTestId('house')).toHaveLength(5)
    expect(within(container).getByText('House E')).toBeInTheDocument()
  })
})
