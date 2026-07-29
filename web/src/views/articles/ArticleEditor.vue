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
import {
  type Article,
  type ArticleCreateInput,
  type ArticleUpdateInput,
  type Category,
  type Tag,
} from '@/api'
import { useTagMutations } from '@/features/tags/use-tag-mutations'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import { renderSafeMarkdown } from '@/shared/lib/safe-markdown'
import { buildTree, getApiError } from '@/utils'
import { isApiError } from '@/shared/api/types'
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

// 乐观锁：保存当前编辑器持有的版本号，保存时作为 expected_version 上送。
// 后端 WHERE version = ? 不匹配返回 409，提示用户刷新后重试。
const currentVersion = ref<number | null>(null)

// Vue Query composables for data fetching
const { data: articleData, refetch: refetchArticle } = useArticleDetailQuery(articleId)
const { data: tagsData } = useTagListQuery({})
const { data: categoriesData } = useCategoryListQuery({})
const { createArticle, updateArticle, publishArticle } = useArticleMutations()
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
    try { renderedContent.value = renderSafeMarkdown(content || '') } catch { renderedContent.value = '' }
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
      featured_image: d.featured_image, status: d.status, post_type: d.post_type, visibility: d.visibility,
      is_pinned: d.is_pinned, is_featured: d.is_featured, allow_comment: d.allow_comment,
      scheduled_at: d.scheduled_at, meta_title: d.meta_title, meta_desc: d.meta_desc,
      meta_keywords: d.meta_keywords,
    })
    // 记录读取时的 version，用于后续保存时的乐观锁校验。
    currentVersion.value = d.version ?? null
  }
}, { immediate: true })

async function saveDraft() {
  await save(false)
}

async function publish() {
  await save(form.status !== 'published')
}

function buildEditablePayload(): Omit<ArticleCreateInput, 'post_type'> {
  return {
    title: form.title,
    slug: form.slug || undefined,
    content: form.content,
    excerpt: form.excerpt,
    category_id: form.category_id,
    tag_ids: form.tag_ids,
    featured_image: form.featured_image,
    visibility: form.visibility,
    password: form.password,
    is_pinned: form.is_pinned,
    is_featured: form.is_featured,
    allow_comment: form.allow_comment,
    meta_title: form.meta_title,
    meta_desc: form.meta_desc,
    meta_keywords: form.meta_keywords,
  }
}

const buildCreatePayload = (): ArticleCreateInput => ({
  ...buildEditablePayload(),
  post_type: form.post_type,
})

const buildUpdatePayload = (): ArticleUpdateInput => buildEditablePayload()

async function save(shouldPublish: boolean) {
  if (!form.title.trim()) {
    ElMessage.warning('请输入文章标题')
    return
  }
  if (isEdit.value && currentVersion.value == null) {
    ElMessage.error('文章版本信息尚未加载，请刷新后重试')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      // 编辑请求必须携带 expected_version，服务端据此拒绝过期写入。
      const updatePayload = {
        id: articleId.value,
        expected_version: currentVersion.value!,
        ...buildUpdatePayload(),
      }
      const res = await updateArticle.mutateAsync(updatePayload)
      // 保存成功后更新本地 version，避免连续保存触发误冲突。
      const newVersion = (res as { data?: { version?: number } })?.data?.version
      if (typeof newVersion === 'number') currentVersion.value = newVersion
      if (shouldPublish) {
        const published = await publishArticle.mutateAsync(articleId.value)
        form.status = published.data.status
      }
      ElMessage.success(shouldPublish ? '文章已发布' : '文章已更新')
    } else {
      const res = await createArticle.mutateAsync(buildCreatePayload())
      if (shouldPublish) {
        const published = await publishArticle.mutateAsync(res.data.id)
        form.status = published.data.status
      }
      ElMessage.success(shouldPublish ? '文章已发布' : '文章已保存')
      const routeName = form.post_type === 'page' ? 'PageEdit' : 'ArticleEdit'
      router.replace({ name: routeName, params: { id: res.data.id } })
    }
  } catch (err) {
    // 409 Conflict: 文章已被他人修改，提示用户刷新获取最新内容。
    if (isApiError(err) && err.status === 409) {
      void ElMessageBox.confirm(
        '该文章已被其他编辑者修改，你的保存未生效。是否重新加载最新内容？未保存的本地改动将被覆盖。',
        '并发冲突',
        { confirmButtonText: '重新加载', cancelButtonText: '稍后手动刷新', type: 'warning' },
      ).then(() => {
        // 触发 Vue Query 重新拉取最新数据，watch 会同步 currentVersion 与表单字段。
        void refetchArticle()
      }).catch(() => { /* 用户选择稍后手动刷新，保持现状 */ })
    } else {
      ElMessage.error(getApiError(err, '保存失败'))
    }
  } finally {
    saving.value = false
  }
}
</script>
