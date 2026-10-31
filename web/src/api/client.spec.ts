import { describe, expect, it } from 'vitest'
import { ApiError, unwrapResponse } from './client'

describe('API client', () => {
  it('unwraps successful envelopes', () => {
    const data = unwrapResponse<{ id: string }>({
      success: true,
      data: { id: 'node-1' }
    })
    expect(data).toEqual({ id: 'node-1' })
  })

  it('throws ApiError on failed envelopes', () => {
    try {
      unwrapResponse({
        success: false,
        error: {
          code: 'NOT_FOUND',
          what: '节点不存在',
          why: 'id 无效'
        }
      })
      throw new Error('should not reach here')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
      const apiError = err as ApiError
      expect(apiError.code).toBe('NOT_FOUND')
      expect(apiError.message).toBe('节点不存在')
      expect(apiError.why).toBe('id 无效')
    }
  })

  it('passes through non-enveloped payloads', () => {
    const payload = { raw: true }
    expect(unwrapResponse(payload)).toEqual(payload)
  })

  it('creates a fallback error when payload is missing', () => {
    const error = ApiError.fromPayload(undefined)
    expect(error.code).toBe('UNKNOWN_ERROR')
    expect(error.message).toContain('请求失败')
  })
})
