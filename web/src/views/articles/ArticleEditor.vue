<template>
  <div class="article-editor">
    <EditorTopbar
      :is-edit="isEdit"
      :saving="saving"
      :status="form.status"
      @back="$router.back()"
      @save-draft="saveDraft"
      @publish="publish"
    />

    <el-row
      :gutter="20"
      class="editor-body"
    >
      <el-col
        :xs="24"
        :lg="16"
      >
        <ArticleMainEditor
          v-model:title="form.title"
          v-model:slug="form.slug"
          v-model:content="form.content"
          v-model:excerpt="form.excerpt"
          v-model:editor-mode="editorMode"
          :rendered-content="renderedContent"
          @auto-slug="autoSlug"
        />
        <ArticleSeoPanel
          v-model:title="form.meta_title"
          v-model:desc="form.meta_desc"
          v-model:keywords="form.meta_keywords"
        />
      </el-col>

      <el-col
        :xs="24"
        :lg="8"
      >
        <ArticleSidebar
          :form="form"
          :all-tags="allTags"
          :category-tree="categoryTree"
          :tree-select-props="treeSelectProps"
          :upload-headers="uploadHeaders"
          @create-tag="createTag"
          @upload-success="handleImageUpload"
        />
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { type Article, type Category, type Tag } from '@/api'
import { useTagMutations } from '@/features/tags/use-tag-mutations'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { renderSafeMarkdown } from '@/shared/lib/safe-markdown'
import { buildTree, getApiError } from '@/utils'
import { useArticleDetailQuery } from '@/features/articles/use-article-detail-query'
import { useArticleMutations } from '@/features/articles/use-article-mutations'
import { useTagListQuery } from '@/features/tags/use-tag-list-query'
import { useCategoryListQuery } from '@/features/categories/use-category-list-query'
import EditorTopbar from './components/EditorTopbar.vue'
import ArticleMainEditor from './components/ArticleMainEditor.vue'
import ArticleSeoPanel from './components/ArticleSeoPanel.vue'
import ArticleSidebar from './components/ArticleSidebar.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const treeSelectProps = { label: 'name', value: 'id', children: 'children' } as const

const articleId = computed(() => Number(route.params.id) || 0)
const isEdit = computed(() => !!route.params.id)
const saving = ref(false)
const editorMode = ref('markdown')

// Vue Query composables for data fetching
const { data: articleData } = useArticleDetailQuery(articleId)
const { data: tagsData } = useTagListQuery({})
const { data: categoriesData } = useCategoryListQuery({})
const { createArticle, updateArticle } = useArticleMutations()
const { createTag: createTagMutation } = useTagMutations()

// Local refs synced from query data (kept mutable for createTag / form population)
const allTags = ref<Tag[]>([])
const categories = ref<Category[]>([])

watch(() => tagsData.value, (v) => { if (v?.data) allTags.value = [...v.data] }, { immediate: true })
watch(() => categoriesData.value, (v) => { if (v?.data) categories.value = [...v.data] }, { immediate: true })

type ArticleStatus = Article['status']
type ArticleVisibility = Article['visibility']
type ArticlePostType = Article['post_type']

const form = reactive({
  title: '',
  slug: '',
  content: '',
  excerpt: '',
  category_id: null as number | null,
  tag_ids: [] as number[],
  featured_image: '',
  status: 'draft' as ArticleStatus,
  post_type: ((route.meta.postType as string) || 'post') as ArticlePostType,
  visibility: 'public' as ArticleVisibility,
  password: '',
  is_pinned: false,
  is_featured: false,
  allow_comment: true,
  scheduled_at: null as Date | null,
  meta_title: '',
  meta_desc: '',
  meta_keywords: '',
})

const categoryTree = computed(() => buildTree(categories.value))

// Preview HTML is rendered debounced instead of via computed: a computed
// would re-run marked + DOMPurify synchronously on every keystroke once the
// preview pane has been opened, which makes typing lag on long articles.
const renderedContent = ref('')
let previewTimer: ReturnType<typeof setTimeout> | null = null

watch(() => form.content, (content) => {
  if (previewTimer) clearTimeout(previewTimer)
  previewTimer = setTimeout(() => {
    previewTimer = null
    try { renderedContent.value = renderSafeMarkdown(content || '') } catch { renderedContent.value = content }
  }, 300)
}, { immediate: true })

onUnmounted(() => {
  if (previewTimer) clearTimeout(previewTimer)
})

const uploadHeaders = computed(() => ({
  Authorization: `Bearer ${authStore.token}`,
}))

function autoSlug() {
  if (!isEdit.value) {
    form.slug = form.title
      .toLowerCase()
      .replace(/[^a-z0-9\u4e00-\u9fff]+/g, '-')
      .replace(/^-|-$/g, '')
  }
}

async function createTag(name: string) {
  try {
    const res = await createTagMutation.mutateAsync({ name })
    allTags.value.push(res.data)
    form.tag_ids.push(res.data.id)
  } catch {
    ElMessage.error('创建标签失败')
  }
}

function handleImageUpload(res: unknown) {
  const data = (res as { data?: { url?: string } })?.data
  if (data?.url) form.featured_image = data.url
}

// Populate form when editing and article data loads
watch(() => articleData.value, (a) => {
  if (a?.data && isEdit.value) {
    const d = a.data
    Object.assign(form, {
      title: d.title, slug: d.slug, content: d.content || '', excerpt: d.excerpt,
      category_id: d.category_id, tag_ids: d.tags?.map((t: any) => t.id) || [],
      featured_image: d.featured_image, status: d.status, visibility: d.visibility,
      is_pinned: d.is_pinned, is_featured: d.is_featured, allow_comment: d.allow_comment,
      scheduled_at: d.scheduled_at, meta_title: d.meta_title, meta_desc: d.meta_desc,
      meta_keywords: d.meta_keywords,
    })
  }
}, { immediate: true })

async function saveDraft() {
  form.status = 'draft'
  await save()
}

async function publish() {
  if (form.status === 'draft') form.status = 'published'
  await save()
}

/** Build a Partial<Article> payload from the reactive form, converting Date → ISO string. */
function buildPayload(): Partial<Article> {
  const { scheduled_at, ...rest } = form
  return {
    ...rest,
    scheduled_at: scheduled_at instanceof Date ? scheduled_at.toISOString() : scheduled_at,
  }
}

async function save() {
  if (!form.title.trim()) {
    ElMessage.warning('请输入文章标题')
    return
  }
  saving.value = true
  try {
    const payload = buildPayload()
    if (isEdit.value) {
      await updateArticle.mutateAsync({ id: articleId.value, ...payload } as any)
      ElMessage.success('文章已更新')
    } else {
      const res = await createArticle.mutateAsync(payload as any)
      ElMessage.success('文章已保存')
      router.replace(`/admin/articles/${(res as any).data.id}/edit`)
    }
  } catch (err) {
    ElMessage.error(getApiError(err, '保存失败'))
  } finally {
    saving.value = false
  }
}
</script>
