import { describe, expect, it } from 'vitest'
import { localized } from './localized'

describe('localized', () => {
  it('picks the current language', () => {
    expect(localized({ uk: 'А', en: 'B' }, 'en', 'uk')).toBe('B')
  })
  it('localized falls back to the default locale', () => {
    expect(localized({ uk: 'А' }, 'en', 'uk')).toBe('А')
  })
  it('falls back to any non-empty value', () => {
    expect(localized({ en: 'B' }, 'uk', 'uk')).toBe('B')
  })
  it('is empty when there is nothing', () => {
    expect(localized({}, 'uk', 'uk')).toBe('')
    expect(localized(undefined, 'uk', 'uk')).toBe('')
  })
  it('treats whitespace-only values as empty', () => {
    expect(localized({ en: '  ', uk: 'А' }, 'en', 'uk')).toBe('А')
  })
})
