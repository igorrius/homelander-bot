import { describe, expect, it } from 'vitest'
import { mockApi } from '../test/api'
import { api, ApiError } from './client'

describe('api client', () => {
  it('ApiError carries validation fields', async () => {
    mockApi({ 'POST /register': { status: 422, body: { error: { code: 'validation', fields: { phone: 'invalid' } } } } })
    const err = await api('/register', { method: 'POST', body: '{}' }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 422, code: 'validation' })
    expect((err as ApiError).fields).toEqual({ phone: 'invalid' })
  })

  it('an answer without an error body is unknown with no fields', async () => {
    mockApi({ 'GET /x': { status: 404 } })
    const err = (await api('/x').catch((e: unknown) => e)) as ApiError
    expect(err.code).toBe('unknown')
    expect(err.fields).toEqual({})
  })
})
