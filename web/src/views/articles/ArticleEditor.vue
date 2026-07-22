<template>
  <div class="article-editor">
    <!-- Top Bar -->
    <div class="editor-topbar">
      <div class="topbar-left">
        <el-button text @click="$router.back()">
          <el-icon><ArrowLeft /></el-icon> 返回
        </el-button>
        <span class="editor-title">{{ isEdit ? '编辑文章' : '写文章' }}</span>
      </div>
      <div class="topbar-right">
        <el-button @click="saveDraft" :loading="saving">保存草稿</el-button>
        <el-button type="primary" @click="publish" :loading="saving">
          {{ form.status === 'published' ? '更新' : '发布' }}
        </el-button>
      </div>
    </div>

    <el-row :gutter="20" class="editor-body">
      <!-- Main Editor -->
      <el-col :xs="24" :lg="16">
        <el-card shadow="never">
          <el-form :model="form" label-position="top">
            <!-- Title -->
            <el-form-item>
              <el-input
                v-model="form.title"
                placeholder="输入文章标题..."
                class="title-input"
                @input="autoSlug"
              />
            </el-form-item>

            <!-- Slug -->
            <el-form-item>
              <el-input v-model="form.slug" placeholder="URL 别名" class="slug-input">
                <template #prepend>slug:</template>
              </el-input>
            </el-form-item>

            <!-- Editor Tabs -->
            <el-form-item>
              <el-tabs v-model="editorMode" class="editor-tabs">
                <el-tab-pane label="Markdown" name="markdown">
                  <div class="markdown-editor">
                    <el-input
                      v-model="form.content"
                      type="textarea"
                      :autosize="{ minRows: 20, maxRows: 50 }"
                      placeholder="使用 Markdown 写作..."
                      class="md-textarea"
                    />
                  </div>
                </el-tab-pane>
                <el-tab-pane label="富文本" name="richtext">
                  <div class="richtext-editor" id="richtext-toolbar">
                    <div class="toolbar">
                      <button @click="execCmd('bold')" title="粗体"><b>B</b></button>
                      <button @click="execCmd('italic')" title="斜体"><i>I</i></button>
                      <button @click="execCmd('underline')" title="下划线"><u>U</u></button>
                      <button @click="execCmd('strikeThrough')" title="删除线"><s>S</s></button>
                      <span class="divider"></span>
                      <button @click="execCmd('formatBlock', 'h1')">H1</button>
                      <button @click="execCmd('formatBlock', 'h2')">H2</button>
                      <button @click="execCmd('formatBlock', 'h3')">H3</button>
                      <span class="divider"></span>
                      <button @click="execCmd('insertUnorderedList')" title="无序列表">• List</button>
                      <button @click="execCmd('insertOrderedList')" title="有序列表">1. List</button>
                      <button @click="execCmd('formatBlock', 'blockquote')" title="引用">❝</button>
                      <button @click="insertCodeBlock()" title="代码块">&lt;/&gt;</button>
                      <span class="divider"></span>
                      <button @click="insertLink()" title="链接">🔗</button>
                      <button @click="insertImage()" title="图片">🖼</button>
                    </div>
                    <div
                      ref="editorRef"
                      class="content-editable"
                      contenteditable="true"
                      @input="onEditorInput"
                      v-html="form.content"
                    ></div>
                  </div>
                </el-tab-pane>
                <el-tab-pane label="预览" name="preview">
                  <div class="preview-pane" v-html="renderedContent"></div>
                </el-tab-pane>
              </el-tabs>
            </el-form-item>

            <!-- Excerpt -->
            <el-form-item label="摘要">
              <el-input
                v-model="form.excerpt"
                type="textarea"
                :rows="3"
                placeholder="文章摘要（留空自动生成）"
              />
            </el-form-item>
          </el-form>
        </el-card>

        <!-- SEO Panel -->
        <el-card shadow="never" class="section-card">
          <template #header><span>SEO 设置</span></template>
          <el-form :model="form" label-position="top">
            <el-form-item label="Meta 标题">
              <el-input v-model="form.meta_title" placeholder="SEO 标题（留空使用文章标题）" />
            </el-form-item>
            <el-form-item label="Meta 描述">
              <el-input v-model="form.meta_desc" type="textarea" :rows="2" placeholder="SEO 描述" />
            </el-form-item>
            <el-form-item label="Meta 关键词">
              <el-input v-model="form.meta_keywords" placeholder="关键词，用逗号分隔" />
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <!-- Sidebar -->
      <el-col :xs="24" :lg="8">
        <!-- Publish Settings -->
        <el-card shadow="never" class="section-card">
          <template #header><span>发布设置</span></template>
          <el-form :model="form" label-position="top" size="small">
            <el-form-item label="状态">
              <el-select v-model="form.status" style="width: 100%">
                <el-option label="草稿" value="draft" />
                <el-option label="已发布" value="published" />
                <el-option label="待审核" value="pending" />
                <el-option label="定时发布" value="scheduled" />
              </el-select>
            </el-form-item>
            <el-form-item label="可见性">
              <el-select v-model="form.visibility" style="width: 100%">
                <el-option label="公开" value="public" />
                <el-option label="私密" value="private" />
                <el-option label="密码保护" value="password" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="form.visibility === 'password'" label="访问密码">
              <el-input v-model="form.password" show-password />
            </el-form-item>
            <el-form-item v-if="form.status === 'scheduled'" label="定时发布">
              <el-date-picker v-model="form.scheduled_at" type="datetime" style="width: 100%" />
            </el-form-item>
            <el-form-item>
              <el-checkbox v-model="form.allow_comment">允许评论</el-checkbox>
            </el-form-item>
            <el-form-item>
              <el-checkbox v-model="form.is_pinned">置顶文章</el-checkbox>
            </el-form-item>
            <el-form-item>
              <el-checkbox v-model="form.is_featured">设为精选</el-checkbox>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- Category -->
        <el-card shadow="never" class="section-card">
          <template #header>
            <div class="card-header-row">
              <span>分类</span>
              <el-button text size="small" type="primary">+ 新建</el-button>
            </div>
          </template>
          <el-tree-select
            v-model="form.category_id"
            :data="categoryTree"
            :props="treeSelectProps"
            placeholder="选择分类"
            check-strictly
            clearable
            style="width: 100%"
          />
        </el-card>

        <!-- Tags -->
        <el-card shadow="never" class="section-card">
          <template #header><span>标签</span></template>
          <el-select
            v-model="form.tag_ids"
            multiple
            filterable
            allow-create
            default-first-option
            placeholder="选择或创建标签"
            style="width: 100%"
            @create="createTag"
          >
            <el-option
              v-for="tag in allTags"
              :key="tag.id"
              :label="tag.name"
              :value="tag.id"
            />
          </el-select>
        </el-card>

        <!-- Featured Image -->
        <el-card shadow="never" class="section-card">
          <template #header><span>特色图片</span></template>
          <div v-if="form.featured_image" class="featured-preview">
            <img :src="form.featured_image" alt="Featured" />
            <el-button text type="danger" @click="form.featured_image = ''">移除</el-button>
          </div>
          <el-upload
            v-else
            action="/api/v1/media/upload"
            :headers="uploadHeaders"
            :on-success="handleImageUpload"
            :show-file-list="false"
            accept="image/*"
          >
            <el-button>上传图片</el-button>
          </el-upload>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { articleApi, categoryApi, tagApi, type Category, type Tag } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const treeSelectProps = { label: 'name', value: 'id', children: 'children' } as any

const isEdit = computed(() => !!route.params.id)
const saving = ref(false)
const editorMode = ref('markdown')
const editorRef = ref<HTMLDivElement>()
const allTags = ref<Tag[]>([])
const categories = ref<Category[]>([])

const form = reactive({
  title: '',
  slug: '',
  content: '',
  excerpt: '',
  category_id: null as number | null,
  tag_ids: [] as number[],
  featured_image: '',
  status: 'draft',
  post_type: (route.meta.postType as string) || 'post',
  visibility: 'public',
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
  try { return marked(form.content || '') } catch { return form.content }
})

const uploadHeaders = computed(() => ({
  Authorization: `Bearer ${authStore.token}`,
}))

function buildTree(items: Category[], parentId: number | null = null): Category[] {
  return items
    .filter(c => c.parent_id === parentId)
    .map(c => ({ ...c, children: buildTree(items, c.id) }))
}

function autoSlug() {
  if (!isEdit.value) {
    form.slug = form.title
      .toLowerCase()
      .replace(/[^a-z0-9\u4e00-\u9fff]+/g, '-')
      .replace(/^-|-$/g, '')
  }
}

function execCmd(command: string, value?: string) {
  document.execCommand(command, false, value || '')
}

function insertCodeBlock() {
  document.execCommand('insertHTML', false, '<pre><code>// code here</code></pre>')
}

function insertLink() {
  const url = prompt('输入链接 URL:')
  if (url) document.execCommand('createLink', false, url)
}

function insertImage() {
  const url = prompt('输入图片 URL:')
  if (url) document.execCommand('insertImage', false, url)
}

function onEditorInput(e: Event) {
  form.content = (e.target as HTMLDivElement).innerHTML
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

function handleImageUpload(res: any) {
  form.featured_image = res.data.url
}

async function saveDraft() {
  form.status = 'draft'
  await save()
}

async function publish() {
  if (form.status === 'draft') form.status = 'published'
  await save()
}

async function save() {
  if (!form.title.trim()) {
    ElMessage.warning('请输入文章标题')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await articleApi.update(Number(route.params.id), form as any)
      ElMessage.success('文章已更新')
    } else {
      const res = await articleApi.create(form as any)
      ElMessage.success('文章已保存')
      router.replace(`/admin/articles/${res.data.id}/edit`)
    }
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '保存失败')
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
      title: a.title, slug: a.slug, content: a.content, excerpt: a.excerpt,
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

<style lang="scss" scoped>
.article-editor {
  .editor-topbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    padding: 12px 0;

    .topbar-left {
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .editor-title {
      font-size: 18px;
      font-weight: 600;
    }
  }

  .title-input :deep(.el-input__inner) {
    font-size: 24px;
    font-weight: 600;
    border: none;
    padding: 0;
  }

  .slug-input :deep(.el-input-group__prepend) {
    background: #f5f7fa;
  }

  .editor-tabs { width: 100%; }

  .markdown-editor .md-textarea :deep(textarea) {
    font-family: 'JetBrains Mono', 'Fira Code', monospace;
    font-size: 14px;
    line-height: 1.8;
    padding: 16px;
  }

  .richtext-editor {
    border: 1px solid #dcdfe6;
    border-radius: 4px;

    .toolbar {
      padding: 8px 12px;
      border-bottom: 1px solid #ebeef5;
      display: flex;
      gap: 4px;
      flex-wrap: wrap;

      button {
        padding: 4px 8px;
        border: 1px solid transparent;
        border-radius: 3px;
        background: none;
        cursor: pointer;
        font-size: 13px;
        &:hover { background: #f5f7fa; border-color: #dcdfe6; }
      }
      .divider {
        width: 1px;
        background: #dcdfe6;
        margin: 0 4px;
      }
    }

    .content-editable {
      min-height: 400px;
      padding: 16px;
      outline: none;
      line-height: 1.8;
      font-size: 15px;

      :deep(h1) { font-size: 28px; margin: 16px 0 8px; }
      :deep(h2) { font-size: 22px; margin: 14px 0 6px; }
      :deep(h3) { font-size: 18px; margin: 12px 0 4px; }
      :deep(blockquote) {
        border-left: 4px solid #409eff;
        padding-left: 16px;
        color: #606266;
        margin: 12px 0;
      }
      :deep(pre) {
        background: #1d1e2c;
        color: #abb2bf;
        padding: 16px;
        border-radius: 6px;
        overflow-x: auto;
      }
    }
  }

  .preview-pane {
    padding: 16px;
    line-height: 1.8;
    font-size: 15px;

    :deep(h1), :deep(h2), :deep(h3) { margin: 20px 0 10px; }
    :deep(img) { max-width: 100%; border-radius: 8px; }
    :deep(code) {
      background: #f5f7fa;
      padding: 2px 6px;
      border-radius: 3px;
      font-size: 13px;
    }
    :deep(pre code) { background: none; padding: 0; }
  }

  .section-card { margin-bottom: 16px; }

  .card-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .featured-preview {
    text-align: center;
    img {
      max-width: 100%;
      border-radius: 6px;
      margin-bottom: 8px;
    }
  }
}
</style>
