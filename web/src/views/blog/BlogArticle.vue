<template>
  <div class="blog-article" ref="pageRef">
    <div v-if="loading" class="loading">Loading...</div>
    <template v-else-if="article">
      <!-- Article Header -->
      <header class="article-header" ref="headerRef">
        <div class="header-meta">
          <router-link v-if="article.category" :to="'/blog/category/' + article.category.slug" class="meta-cat">{{ article.category.name }}</router-link>
          <span>{{ formatDate(article.created_at) }}</span>
          <span>{{ article.view_count || 0 }} views</span>
        </div>
        <h1>{{ article.title }}</h1>
        <p class="header-summary" v-if="article.summary">{{ article.summary }}</p>
        <div class="header-author" v-if="article.author">
          <el-avatar :size="32">{{ (article.author.display_name || 'U')[0] }}</el-avatar>
          <span>{{ article.author.display_name }}</span>
        </div>
      </header>

      <!-- Article Content -->
      <article class="article-content markdown-body" v-html="renderedContent"></article>

      <!-- Tags -->
      <div class="article-tags" v-if="article.tags && article.tags.length">
        <router-link v-for="tag in article.tags" :key="tag.id" :to="'/blog/tag/' + tag.slug" class="tag-chip">{{ tag.name }}</router-link>
      </div>

      <!-- Like -->
      <div class="article-actions">
        <button class="like-btn" :class="{ liked }" @click="toggleLike">
          {{ liked ? 'Liked' : 'Like' }} ({{ article.like_count || 0 }})
        </button>
      </div>

      <!-- Comments -->
      <section class="comments-section">
        <h3>Comments ({{ comments.length }})</h3>
        <div class="comment-form">
          <input v-model="commentName" placeholder="Your name" class="comment-input" />
          <textarea v-model="commentBody" placeholder="Leave a comment..." class="comment-textarea" rows="3"></textarea>
          <button class="comment-submit" @click="submitComment" :disabled="!commentBody.trim()">Submit</button>
        </div>
        <div class="comment-list">
          <div v-for="c in comments" :key="c.id" class="comment-item">
            <div class="comment-avatar">{{ (c.author_name || 'A')[0].toUpperCase() }}</div>
            <div class="comment-body">
              <div class="comment-head"><strong>{{ c.author_name || 'Anonymous' }}</strong><span>{{ formatDate(c.created_at) }}</span></div>
              <p>{{ c.content }}</p>
            </div>
          </div>
        </div>
      </section>
    </template>
    <div v-else class="not-found"><h2>Article not found</h2><router-link to="/blog">Back to articles</router-link></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import { animate } from 'animejs'
import dayjs from 'dayjs'

const route = useRoute()
const article = ref<any>(null)
const comments = ref<any[]>([])
const loading = ref(true)
const liked = ref(false)
const commentName = ref('')
const commentBody = ref('')
const pageRef = ref<HTMLElement>()
const headerRef = ref<HTMLElement>()

const renderedContent = computed(() => {
  if (!article.value?.content) return ''
  return marked(article.value.content) as string
})

function formatDate(s: string) { return dayjs(s).format('MMM DD, YYYY') }

async function fetchArticle() {
  loading.value = true
  try {
    const slug = route.params.slug as string
    const res = await fetch('/api/v1/articles/slug/' + slug)
    if (!res.ok) throw new Error('not found')
    article.value = await res.json()
    if (article.value?.id) fetchComments()
  } catch { article.value = null }
  finally { loading.value = false }
}

async function fetchComments() {
  try {
    const res = await fetch('/api/v1/articles/' + article.value.id + '/comments')
    const data = await res.json()
    comments.value = data.items || data.data || []
  } catch {}
}

async function submitComment() {
  if (!commentBody.value.trim() || !article.value) return
  try {
    await fetch('/api/v1/comments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        article_id: article.value.id,
        author_name: commentName.value || 'Anonymous',
        content: commentBody.value,
      }),
    })
    commentBody.value = ''
    fetchComments()
  } catch {}
}

async function toggleLike() {
  if (!article.value) return
  try {
    await fetch('/api/v1/articles/' + article.value.id + '/like', { method: 'POST' })
    liked.value = !liked.value
    article.value.like_count = (article.value.like_count || 0) + (liked.value ? 1 : -1)
  } catch {}
}

onMounted(() => {
  fetchArticle()
  if (headerRef.value) {
    animate(headerRef.value, { opacity: { from: 0 }, translateY: { from: 20 }, duration: 700, ease: 'outQuint' })
  }
})
</script>

<style lang="scss" scoped>
.blog-article { max-width: 800px; margin: 0 auto; }
.loading { text-align: center; padding: 60px; color: var(--text-muted, #909399); }
.article-header { margin-bottom: 32px; }
.header-meta { display: flex; gap: 16px; align-items: center; font-size: 13px; color: var(--text-muted, #909399); margin-bottom: 12px; }
.meta-cat { color: #409eff; text-decoration: none; font-weight: 600; }
.meta-cat:hover { text-decoration: underline; }
.article-header h1 { font-size: 32px; font-weight: 700; line-height: 1.3; margin-bottom: 12px; color: var(--text-primary, #303133); }
.header-summary { font-size: 16px; color: var(--text-secondary, #606266); line-height: 1.6; margin-bottom: 16px; }
.header-author { display: flex; align-items: center; gap: 10px; font-size: 14px; color: var(--text-secondary, #606266); }
.article-content { margin-bottom: 32px; }
.article-tags { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 24px; }
.tag-chip { display: inline-block; padding: 4px 14px; background: var(--bg-primary, #f5f7fa); border-radius: 16px; font-size: 13px; color: var(--text-secondary, #606266); text-decoration: none; transition: all 0.2s; }
.tag-chip:hover { background: #409eff; color: #fff; }
.article-actions { margin-bottom: 32px; }
.like-btn { padding: 10px 28px; border: 1px solid var(--border-color, #ebeef5); background: var(--bg-card, #fff); border-radius: 8px; cursor: pointer; font-size: 14px; color: var(--text-secondary, #606266); transition: all 0.2s; }
.like-btn:hover { border-color: #f56c6c; color: #f56c6c; }
.like-btn.liked { background: #f56c6c; color: #fff; border-color: #f56c6c; }
.comments-section { border-top: 1px solid var(--border-color, #ebeef5); padding-top: 24px; }
.comments-section h3 { font-size: 18px; margin-bottom: 20px; color: var(--text-primary, #303133); }
.comment-form { display: flex; flex-direction: column; gap: 10px; margin-bottom: 24px; }
.comment-input, .comment-textarea { padding: 10px 14px; border: 1px solid var(--border-color, #ebeef5); border-radius: 8px; font-size: 14px; background: var(--bg-card, #fff); color: var(--text-primary, #303133); font-family: inherit; }
.comment-textarea { resize: vertical; }
.comment-submit { align-self: flex-end; padding: 8px 24px; background: #409eff; color: #fff; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; transition: 0.2s; }
.comment-submit:hover { background: #337ecc; }
.comment-submit:disabled { opacity: 0.5; cursor: default; }
.comment-list { display: flex; flex-direction: column; gap: 16px; }
.comment-item { display: flex; gap: 12px; }
.comment-avatar { width: 36px; height: 36px; border-radius: 50%; background: #409eff; color: #fff; display: flex; align-items: center; justify-content: center; font-size: 14px; font-weight: 600; flex-shrink: 0; }
.comment-body { flex: 1; }
.comment-head { display: flex; gap: 12px; align-items: center; margin-bottom: 4px; font-size: 14px; }
.comment-head span { color: var(--text-muted, #909399); font-size: 12px; }
.comment-body p { font-size: 14px; color: var(--text-secondary, #606266); line-height: 1.6; }
.not-found { text-align: center; padding: 80px 20px; }
.not-found h2 { margin-bottom: 12px; }
.not-found a { color: #409eff; }

@media (max-width: 600px) {
  .article-header h1 { font-size: 24px; }
}
</style>
