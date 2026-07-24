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
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { articleApi, categoryApi, tagApi, type Article, type Category, type Tag } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { buildTree, getApiError } from '@/utils'
import EditorTopbar from './components/EditorTopbar.vue'
import ArticleMainEditor from './components/ArticleMainEditor.vue'
import ArticleSeoPanel from './components/ArticleSeoPanel.vue'
import ArticleSidebar from './components/ArticleSidebar.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const treeSelectProps = { label: 'name', value: 'id', children: 'children' } as const

const isEdit = computed(() => !!route.params.id)
const saving = ref(false)
const editorMode = ref('markdown')
const allTags = ref<Tag[]>([])
const categories = ref<Category[]>([])

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

const renderedContent = computed(() => {
  try { return DOMPurify.sanitize(marked(form.content || '') as string) } catch { return form.content }
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
    const res = await tagApi.create({ name })
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
      await articleApi.update(Number(route.params.id), payload)
      ElMessage.success('文章已更新')
    } else {
      const res = await articleApi.create(payload)
      ElMessage.success('文章已保存')
      router.replace(`/admin/articles/${res.data.id}/edit`)
    }
  } catch (err) {
    ElMessage.error(getApiError(err, '保存失败'))
  } finally {
    saving.value = false
  }
}

async function fetchData() {
  const [tagRes, catRes] = await Promise.all([tagApi.list(), categoryApi.list()])
  allTags.value = tagRes.data
  categories.value = catRes.data

  if (isEdit.value) {
    const res = await articleApi.get(Number(route.params.id))
    const a = res.data
    Object.assign(form, {
      title: a.title, slug: a.slug, content: DOMPurify.sanitize(a.content || ''), excerpt: a.excerpt,
      category_id: a.category_id, tag_ids: a.tags?.map(t => t.id) || [],
      featured_image: a.featured_image, status: a.status, visibility: a.visibility,
      is_pinned: a.is_pinned, is_featured: a.is_featured, allow_comment: a.allow_comment,
      scheduled_at: a.scheduled_at, meta_title: a.meta_title, meta_desc: a.meta_desc,
      meta_keywords: a.meta_keywords,
    })
  }
}

onMounted(fetchData)
</script>
