import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import en from '../locales/en.json'
import uk from '../locales/uk.json'
import { renderWithProviders } from '../test/render'
import { BackLink } from './BackLink'

describe('BackLink', () => {
  it('draws an arrow icon instead of a text glyph', () => {
    renderWithProviders(<BackLink to="/admin/site?tab=pages">Сайт</BackLink>)
    const link = screen.getByRole('link', { name: 'Сайт' })
    expect(link).toHaveAttribute('href', '/admin/site?tab=pages')
    expect(link.querySelector('svg')).not.toBeNull()
  })

  it('has no arrow glyphs in the back link texts', () => {
    for (const l of [uk, en]) {
      expect(l.admin.pages.back).not.toMatch(/[←→]/)
      expect(l.admin.data.back).not.toMatch(/[←→]/)
    }
  })
})
