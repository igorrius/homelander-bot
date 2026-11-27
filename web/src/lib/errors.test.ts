import { beforeEach, describe, expect, it } from 'vitest'
import { ApiError } from '../api/client'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { errorText, fieldErrors } from './errors'

describe('error translation', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })
  const t = i18n.t.bind(i18n)

  it('errorText of a known code', () => {
    expect(errorText(t, new ApiError(429, 'rate_limited'))).toBe(uk.errors.rate_limited)
  })
  it('errorText of an unknown code is the generic text', () => {
    expect(errorText(t, new ApiError(500, 'whatever'))).toBe(uk.errors.unknown)
    expect(errorText(t, new TypeError('Failed to fetch'))).toBe(uk.errors.unknown)
  })
  it('fieldErrors translates reasons', () => {
    const err = new ApiError(422, 'validation', { phone: 'taken', name: 'weird' })
    expect(fieldErrors(t, err)).toEqual({ phone: uk.validation.taken, name: uk.validation.invalid })
  })
  it('fieldErrors of other errors is empty', () => {
    expect(fieldErrors(t, new ApiError(429, 'rate_limited'))).toEqual({})
    expect(fieldErrors(t, null)).toEqual({})
  })
})
