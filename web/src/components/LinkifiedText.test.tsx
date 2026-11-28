import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { renderWithProviders } from '../test/render'
import { splitLinks } from '../lib/links'
import { LinkifiedText } from './LinkifiedText'

describe('LinkifiedText', () => {
  it('turns http and https URLs into links that open in a new tab', () => {
    renderWithProviders(<LinkifiedText text={'See https://example.com/a?b=1 and http://example.com.'} />)
    const a = screen.getByRole('link', { name: 'https://example.com/a?b=1' })
    expect(a).toHaveAttribute('href', 'https://example.com/a?b=1')
    expect(a).toHaveAttribute('target', '_blank')
    expect(a).toHaveAttribute('rel', 'noopener noreferrer')
    expect(screen.getByRole('link', { name: 'http://example.com' })).toHaveAttribute('href', 'http://example.com')
  })

  it('keeps line breaks', () => {
    renderWithProviders(<LinkifiedText text={'line one\nline two'} />)
    const el = screen.getByText(/line one/)
    expect(el.textContent).toBe('line one\nline two')
    expect(el).toHaveStyle({ whiteSpace: 'pre-wrap' })
  })

  it('does not link other schemes', () => {
    renderWithProviders(<LinkifiedText text={'javascript:alert(1) ftp://example.com www.example.com'} />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})

describe('splitLinks', () => {
  it('leaves trailing punctuation out of the link', () => {
    expect(splitLinks('(see https://example.com/x), then https://example.com/y.')).toEqual([
      { text: '(see ' },
      { text: 'https://example.com/x', href: 'https://example.com/x' },
      { text: '), then ' },
      { text: 'https://example.com/y', href: 'https://example.com/y' },
      { text: '.' },
    ])
  })

  it('keeps a closing parenthesis that belongs to the URL', () => {
    expect(splitLinks('https://example.com/wiki/A_(b)!')).toEqual([
      { text: 'https://example.com/wiki/A_(b)', href: 'https://example.com/wiki/A_(b)' },
      { text: '!' },
    ])
  })

  it('returns plain text unchanged', () => {
    expect(splitLinks('no links here')).toEqual([{ text: 'no links here' }])
    expect(splitLinks('')).toEqual([])
  })
})
