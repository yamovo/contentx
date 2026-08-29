import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ContentX } from './client.js'

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

describe('ContentX response handling', () => {
  const fetchMock = vi.fn<Parameters<typeof fetch>, ReturnType<typeof fetch>>()
  let client: ContentX

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
    client = new ContentX({ baseURL: 'https://contentx.test/api/v1', token: 'test-token' })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns the business data from a successful API envelope', async () => {
    const article = { id: 42, title: 'Envelope contract' }
    fetchMock.mockResolvedValueOnce(jsonResponse({
      code: 0,
      message: 'success',
      data: article,
    }))

    await expect(client.articles.get(42)).resolves.toEqual(article)
    expect(fetchMock).toHaveBeenCalledWith(
      'https://contentx.test/api/v1/articles/42',
      expect.objectContaining({
        method: 'GET',
        headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
      }),
    )
  })

  it('uses message and err_code from an HTTP error envelope', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({
      code: -1,
      message: 'Article not found',
      err_code: 'NOT_FOUND',
    }, { status: 404, statusText: 'Not Found' }))

    await expect(client.articles.get(404)).rejects.toMatchObject({
      name: 'ContentXError',
      message: 'Article not found',
      status: 404,
      code: 'NOT_FOUND',
    })
  })

  it('rejects a non-zero application code even when HTTP succeeds', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({
      code: -1,
      message: 'Operation rejected',
      err_code: 'CONFLICT',
    }))

    await expect(client.articles.get(1)).rejects.toMatchObject({
      message: 'Operation rejected',
      status: 200,
      code: 'CONFLICT',
    })
  })

  it.each([200, 204])('returns undefined for a successful %s response without a body', async (status: number) => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status }))

    await expect(client.articles.delete(1)).resolves.toBeUndefined()
  })

  it('preserves the bare JSON health response used by the backend', async () => {
    const health = { status: 'healthy', database: true }
    fetchMock.mockResolvedValueOnce(jsonResponse(health))

    await expect(client.system.health()).resolves.toEqual(health)
  })

  it('applies configured and updated tenant IDs to JSON and upload requests', async () => {
    client = new ContentX({
      baseURL: 'https://contentx.test/api/v1',
      tenantID: 7,
    })
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ code: 0, message: 'success', data: [] }))
      .mockResolvedValueOnce(jsonResponse({
        code: 0,
        message: 'created',
        data: { id: 9, filename: 'asset.txt' },
      }))
      .mockResolvedValueOnce(jsonResponse({ status: 'healthy', database: true }))

    await client.articles.list()
    expect(fetchMock.mock.calls[0]?.[1]?.headers).toMatchObject({ 'X-Tenant-ID': '7' })

    client.setTenantID(12)
    await client.media.upload(new Blob(['asset']), 'asset.txt')
    expect(fetchMock.mock.calls[1]?.[1]?.headers).toMatchObject({ 'X-Tenant-ID': '12' })

    client.clearTenantID()
    await client.system.health()
    expect(fetchMock.mock.calls[2]?.[1]?.headers).not.toHaveProperty('X-Tenant-ID')
  })

  it('does not hide a network exception', async () => {
    const networkError = new TypeError('fetch failed')
    fetchMock.mockRejectedValueOnce(networkError)

    await expect(client.articles.list()).rejects.toBe(networkError)
  })
})
