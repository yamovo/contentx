<template>
  <div class="blog-list" ref="pageRef">
    <div class="list-header" ref="headerRef">
      <h1>{{ headerTitle }}</h1>
      <p class="header-desc">{{ headerDesc }}</p>
    </div>
    <div class="list-body">
      <div class="articles-col">
        <div v-for="article in articles" :key="article.id" class="article-card">
          <div class="card-image" v-if="article.featured_image">
            <img :src="article.featured_image" :alt="article.title" loading="lazy" />
          </div>
          <div class="card-body">
            <div class="card-meta">
              <router-link v-if="getCategory(article.category_id)" :to="'/blog/category/' + getCategory(article.category_id).slug" class="meta-cat">{{ getCategory(article.category_id).name }}</router-link>
              <span class="meta-date">{{ formatDate(article.created_at) }}</span>
            </div>
            <router-link :to="'/blog/article/' + (article.slug || article.id)" class="card-title">{{ article.title }}</router-link>
            <p class="card-summary">{{ article.excerpt || truncate(article.content, 160) }}</p>
            <div class="card-footer">
              <div class="card-tags" v-if="article.tags && article.tags.length">
                <router-link v-for="tag in article.tags.slice(0, 3)" :key="tag.id" :to="'/blog/tag/' + tag.slug" class="tag-chip">{{ tag.name }}</router-link>
              </div>
              <div class="card-stats">
                <span>{{ article.view_count || 0 }} views</span>
                <span>{{ article.like_count || 0 }} likes</span>
              </div>
            </div>
          </div>
        </div>
        <div v-if="!loading && articles.length === 0" class="empty-state"><p>No articles found.</p></div>
        <div class="pagination" v-if="totalPages > 1">
          <button :disabled="page <= 1" @click="goPage(page - 1)">Prev</button>
          <span>{{ page }} / {{ totalPages }}</span>
          <button :disabled="page >= totalPages" @click="goPage(page + 1)">Next</button>
        </div>
      </div>
      <aside class="sidebar-col">
        <div class="sidebar-card">
          <h3>Categories</h3>
          <div class="cat-list">
            <router-link v-for="cat in categories" :key="cat.id" :to="'/blog/category/' + cat.slug" class="cat-item">
              <span>{{ cat.name }}</span><span class="cat-count">{{ cat.post_count || 0 }}</span>
            </router-link>
          </div>
        </div>
        <div class="sidebar-card">
          <h3>Tags</h3>
          <div class="tag-cloud">
            <router-link v-for="tag in tags" :key="tag.id" :to="'/blog/tag/' + tag.slug" class="tag-chip">{{ tag.name }}</router-link>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { animate } from 'animejs'
import { stagger } from 'animejs/utils'
import dayjs from 'dayjs'

const route = useRoute()
const router = useRouter()
const articles = ref<any[]>([])
const categories = ref<any[]>([])
const tags = ref<any[]>([])
const page = ref(1)
const totalPages = ref(1)
const loading = ref(false)
const pageRef = ref<HTMLElement>()
const headerRef = ref<HTMLElement>()

const isCategory = computed(() => !!route.params.categorySlug)
const isTag = computed(() => !!route.params.tagSlug)
const headerTitle = computed(() => {
  if (isCategory.value) return 'Category: ' + route.params.categorySlug
  if (isTag.value) return 'Tag: ' + route.params.tagSlug
  return 'Articles'
})
const headerDesc = computed(() => {
  if (isCategory.value) return 'Browse articles in this category'
  if (isTag.value) return 'Browse articles tagged with this'
  return 'Latest articles from ContentX'
})

function truncate(s: string, n: number) { return !s ? '' : s.length > n ? s.slice(0, n) + '...' : s }
function formatDate(s: string) { return dayjs(s).format('MMM DD, YYYY') }
function getCategory(id: number) { return categories.value.find(c => c.id === id) }

function goPage(p: number) { page.value = p; fetchArticles(); window.scrollTo({ top: 0, behavior: 'smooth' }) }

async function fetchArticles() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    params.set('page', String(page.value))
    params.set('page_size', '9')
    params.set('status', 'published')
    if (isCategory.value) params.set('category_slug', route.params.categorySlug as string)
    if (isTag.value) params.set('tag_slug', route.params.tagSlug as string)
    const res = await fetch('/api/v1/articles?' + params.toString())
    const data = await res.json()
    articles.value = data.items || []
    totalPages.value = data.total_pages || 1
  } catch { articles.value = [] }
  finally { loading.value = false; await nextTick(); animateCards() }
}

async function fetchSidebar() {
  try {
    const [catRes, tagRes] = await Promise.all([
      fetch('/api/v1/categories?page=1&page_size=50'),
      fetch('/api/v1/tags?page=1&page_size=30'),
    ])
    categories.value = (await catRes.json()).data || []
    tags.value = (await tagRes.json()).data || []
  } catch {}
}

function animateCards() {
  animate('.article-card', {
    opacity: { from: 0 }, translateY: { from: 24 },
    duration: 500, delay: stagger(60), ease: 'outQuint',
  })
}

onMounted(() => {
  if (headerRef.value) {
    animate(headerRef.value, { opacity: { from: 0 }, translateY: { from: 16 }, duration: 600, ease: 'outQuint' })
  }
  fetchArticles()
  fetchSidebar()
})

watch(() => route.params, () => { page.value = 1; fetchArticles() }, { deep: true })
</script>

<style lang="scss" scoped>
.blog-list {
  .list-header { margin-bottom: 32px; }
  .list-header h1 { font-size: 28px; font-weight: 700; margin-bottom: 8px; color: var(--text-primary, #303133); }
  .list-header .header-desc { color: var(--text-muted, #909399); font-size: 15px; }
  .list-body { display: grid; grid-template-columns: 1fr 300px; gap: 32px; align-items: start; }
  .articles-col { display: flex; flex-direction: column; gap: 20px; }
  .article-card { background: var(--bg-card, #fff); border-radius: 12px; overflow: hidden; border: 1px solid var(--border-color, #ebeef5); transition: transform 0.2s, box-shadow 0.2s; display: flex; flex-direction: row; }
  .article-card:hover { transform: translateY(-2px); box-shadow: 0 8px 24px rgba(0,0,0,0.06); }
  .card-image { width: 220px; min-height: 160px; flex-shrink: 0; }
  .card-image img { width: 100%; height: 100%; object-fit: cover; }
  .card-body { padding: 20px; flex: 1; display: flex; flex-direction: column; }
  .card-meta { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; font-size: 13px; }
  .meta-cat { color: #409eff; text-decoration: none; font-weight: 600; }
  .meta-cat:hover { text-decoration: underline; }
  .meta-date { color: var(--text-muted, #909399); }
  .card-title { font-size: 18px; font-weight: 600; color: var(--text-primary, #303133); text-decoration: none; margin-bottom: 8px; line-height: 1.4; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .card-title:hover { color: #409eff; }
  .card-summary { font-size: 14px; color: var(--text-secondary, #606266); line-height: 1.6; margin-bottom: 12px; flex: 1; display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
  .card-footer { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
  .card-tags { display: flex; gap: 6px; flex-wrap: wrap; }
  .tag-chip { display: inline-block; padding: 2px 10px; background: var(--bg-primary, #f5f7fa); border-radius: 12px; font-size: 12px; color: var(--text-secondary, #606266); text-decoration: none; transition: all 0.2s; }
  .tag-chip:hover { background: #409eff; color: #fff; }
  .card-stats { display: flex; gap: 12px; font-size: 12px; color: var(--text-muted, #909399); }
  .sidebar-col { display: flex; flex-direction: column; gap: 20px; position: sticky; top: 92px; }
  .sidebar-card { background: var(--bg-card, #fff); border: 1px solid var(--border-color, #ebeef5); border-radius: 12px; padding: 20px; }
  .sidebar-card h3 { font-size: 15px; font-weight: 600; margin-bottom: 16px; color: var(--text-primary, #303133); }
  .cat-list { display: flex; flex-direction: column; gap: 4px; }
  .cat-item { display: flex; justify-content: space-between; align-items: center; padding: 8px 10px; border-radius: 6px; text-decoration: none; color: var(--text-secondary, #606266); font-size: 14px; transition: all 0.2s; }
  .cat-item:hover { background: var(--bg-primary, #f5f7fa); color: #409eff; }
  .cat-count { background: var(--bg-primary, #f5f7fa); padding: 1px 8px; border-radius: 10px; font-size: 12px; }
  .tag-cloud { display: flex; flex-wrap: wrap; gap: 8px; }
  .empty-state { text-align: center; padding: 60px 20px; color: var(--text-muted, #909399); }
  .pagination { display: flex; justify-content: center; align-items: center; gap: 16px; padding: 20px 0; }
  .pagination button { padding: 8px 20px; border: 1px solid var(--border-color, #ebeef5); background: var(--bg-card, #fff); border-radius: 6px; cursor: pointer; color: var(--text-primary, #303133); font-size: 14px; transition: all 0.2s; }
  .pagination button:hover:not(:disabled) { border-color: #409eff; color: #409eff; }
  .pagination button:disabled { opacity: 0.4; cursor: default; }
  .pagination span { font-size: 14px; color: var(--text-muted, #909399); }
}

@media (max-width: 900px) {
  .blog-list .list-body { grid-template-columns: 1fr; }
  .blog-list .sidebar-col { position: static; order: -1; }
  .blog-list .article-card { flex-direction: column; }
  .blog-list .card-image { width: 100%; min-height: 180px; max-height: 200px; }
}
@media (max-width: 480px) {
  .blog-list .list-header h1 { font-size: 22px; }
  .blog-list .card-title { font-size: 16px; }
}
</style>
