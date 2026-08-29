# `@contentx/sdk`

TypeScript SDK for the ContentX REST API. It unwraps ContentX API responses and returns the business value from `data` directly.

## Install

```bash
npm install @contentx/sdk
```

## Usage

```ts
import { ContentX, ContentXError } from '@contentx/sdk'

const contentx = new ContentX({
  baseURL: 'https://cms.example.com/api/v1',
  token: process.env.CONTENTX_TOKEN,
  tenantID: 12,
})

try {
  const page = await contentx.articles.list({ page: 1, page_size: 20 })
  console.log(page.items)
} catch (error) {
  if (error instanceof ContentXError) {
    console.error(error.status, error.code, error.message)
  }
}
```

Successful API envelopes such as `{ "code": 0, "message": "success", "data": { ... } }` are unwrapped. Failed envelopes throw `ContentXError`; its `code` property contains the backend `err_code` value.

Use `setToken()` when credentials rotate. Use `setTenantID()` to change the `X-Tenant-ID` header and `clearTenantID()` to stop sending an explicit tenant override.

## Development

```bash
npm ci
npm run check
npm pack --dry-run
```

`npm run check` runs strict type checking, unit tests, and a clean dual ESM/CommonJS build. `prepack` rebuilds `dist` so published entry points cannot be stale.
