import { ContentX, ContentXError } from '@contentx/sdk'

const client = new ContentX({
  baseURL: 'http://localhost:8080/api/v1',
  token: '<api-token>',
  tenantID: 1,
})

try {
  const articles = await client.articles.list({ page: 1, page_size: 10 })
  for (const article of articles.items) {
    console.log(article.id, article.title)
  }
} catch (error) {
  if (error instanceof ContentXError) {
    console.error(`ContentX ${error.code ?? error.status}: ${error.message}`)
  } else {
    throw error
  }
}
