<template>
  <div class="entry-page">
    <div class="page-header">
      <el-button
        text
        @click="$router.push('/admin/content-types')"
      >
        <el-icon><ArrowLeft /></el-icon> 内容类型
      </el-button>
      <h2>{{ contentType?.name || uid }}</h2>
      <el-tag
        v-if="contentType?.is_single"
        type="warning"
        size="small"
      >
        单例
      </el-tag>
      <div class="header-actions">
        <el-button @click="exportEntries">
          <el-icon><Download /></el-icon> 导出
        </el-button>
        <el-button @click="importVisible = true">
          <el-icon><Upload /></el-icon> 导入
        </el-button>
        <el-button
          type="primary"
          @click="openEditor()"
        >
          <el-icon><Plus /></el-icon> 新建条目
        </el-button>
      </div>
    </div>

    <el-card shadow="never">
      <div class="filter-bar">
        <el-input
          v-model="search"
          placeholder="搜索条目内容"
          clearable
          style="width: 240px"
          @keyup.enter="fetchEntries"
          @clear="fetchEntries"
        />
        <el-select
          v-if="contentType?.draft_publish"
          v-model="statusFilter"
          style="width: 130px"
          @change="fetchEntries"
        >
          <el-option
            value=""
            label="全部状态"
          />
          <el-option
            value="draft"
            label="草稿"
          />
          <el-option
            value="published"
            label="已发布"
          />
        </el-select>
      </div>

      <el-table
        v-loading="loading"
        :data="entries"
      >
        <el-table-column
          v-for="field in displayFields"
          :key="field.name"
          :label="field.label"
          min-width="140"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            {{ formatValue(row.data?.[field.name]) }}
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="90"
        >
          <template #default="{ row }">
            <el-tag
              :type="row.status === 'published' ? 'success' : 'info'"
              size="small"
            >
              {{ row.status === 'published' ? '已发布' : '草稿' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="语言"
          prop="locale"
          width="70"
        />
        <el-table-column
          label="更新时间"
          width="170"
        >
          <template #default="{ row }">
            {{ new Date(row.updated_at).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="270"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              text
              size="small"
              @click="openEditor(row as ContentEntry)"
            >
              编辑
            </el-button>
            <el-button
              text
              size="small"
              @click="showTranslations(row as ContentEntry)"
            >
              翻译
            </el-button>
            <el-button
              v-if="contentType?.draft_publish && row.status === 'draft'"
              text
              size="small"
              type="success"
              @click="publish(row as ContentEntry)"
            >
              发布
            </el-button>
            <el-button
              v-else-if="contentType?.draft_publish"
              text
              size="small"
              type="warning"
              @click="unpublish(row as ContentEntry)"
            >
              下线
            </el-button>
            <el-popconfirm
              title="确认删除该条目？"
              @confirm="removeEntry(row as ContentEntry)"
            >
              <template #reference>
                <el-button
                  text
                  size="small"
                  type="danger"
                >
                  删除
                </el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-if="total > pageSize"
        v-model:current-page="page"
        class="pagination"
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        @current-change="fetchEntries"
      />
    </el-card>

    <ContentEntryEditorDrawer
      v-model="editorVisible"
      :entry="editingEntry"
      :fields="sortedFields"
      :saving="saving"
      @submit="saveEntry"
    />

    <!-- Import dialog -->
    <el-dialog
      v-model="importVisible"
      title="导入条目"
      width="560px"
    >
      <el-input
        v-model="importJson"
        type="textarea"
        :rows="10"
        placeholder="粘贴导出的 JSON 数据"
      />
      <template #footer>
        <el-button @click="importVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="importing"
          @click="importEntries"
        >
          导入
        </el-button>
      </template>
    </el-dialog>

    <!-- Translations drawer -->
    <el-drawer
      v-model="translationsVisible"
      title="多语言翻译"
      size="480px"
    >
      <div
        v-if="translationSource"
        class="translation-source"
      >
        源条目语言：<el-tag size="small">
          {{ translationSource.locale }}
        </el-tag>
      </div>

      <el-table
        v-loading="translationsLoading"
        :data="translations"
      >
        <el-table-column
          label="语言"
          prop="locale"
          width="80"
        />
        <el-table-column
          label="状态"
          width="90"
        >
          <template #default="{ row }">
            <el-tag
              :type="row.status === 'published' ? 'success' : 'info'"
              size="small"
            >
              {{ row.status === 'published' ? '已发布' : '草稿' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="更新时间"
          min-width="150"
        >
          <template #default="{ row }">
            {{ new Date(row.updated_at).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="80"
        >
          <template #default="{ row }">
            <el-button
              text
              size="small"
              @click="editTranslation(row as ContentEntry)"
            >
              编辑
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-divider content-position="left">
        新建翻译
      </el-divider>
      <div class="translation-create">
        <el-select
          v-model="newLocale"
          filterable
          allow-create
          default-first-option
          placeholder="目标语言 (BCP-47)"
          style="width: 200px"
        >
          <el-option
            v-for="l in localeOptions"
            :key="l"
            :value="l"
            :label="l"
          />
        </el-select>
        <el-button
          type="primary"
          :loading="translating"
          @click="createTranslation"
        >
          创建
        </el-button>
      </div>
      <div class="translation-hint">
        新翻译会复制源条目的内容作为草稿，创建后可在列表中编辑。
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { contentApi, type ContentType, type ContentField, type ContentEntry } from '@/api'
import { ElMessage } from 'element-plus'
import ContentEntryEditorDrawer from '@/features/content-entry/ContentEntryEditorDrawer.vue'
import {
  queryContentEntries,
  queryContentTranslations,
  queryContentType,
} from '@/features/content-entry/queries'

const route = useRoute()
const uid = computed(() => route.params.uid as string)

const contentType = ref<ContentType | null>(null)
const entries = ref<ContentEntry[]>([])
const loading = ref(false)
const search = ref('')
const statusFilter = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)

const editorVisible = ref(false)
const saving = ref(false)
const editingEntry = ref<ContentEntry | null>(null)

const importVisible = ref(false)
const importing = ref(false)
const importJson = ref('')

const translationsVisible = ref(false)
const translationsLoading = ref(false)
const translating = ref(false)
const translations = ref<ContentEntry[]>([])
const translationSource = ref<ContentEntry | null>(null)
const newLocale = ref('')
const localeOptions = ['en', 'zh', 'ja', 'ko', 'fr', 'de', 'es']

const sortedFields = computed<ContentField[]>(() =>
  [...(contentType.value?.fields || [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0)),
)

// Table shows the first few fields to stay readable.
const displayFields = computed(() => sortedFields.value.slice(0, 4))

function formatValue(v: unknown): string {
  if (v === null || v === undefined || v === '') return '-'
  if (typeof v === 'boolean') return v ? '是' : '否'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

async function fetchType() {
  contentType.value = await queryContentType(uid.value)
}

async function fetchEntries() {
  loading.value = true
  try {
    const res = await queryContentEntries(uid.value, {
      page: page.value,
      page_size: pageSize,
      search: search.value || undefined,
      status: statusFilter.value || undefined,
    })
    entries.value = res.items || []
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function openEditor(entry?: ContentEntry) {
  editingEntry.value = entry || null
  editorVisible.value = true
}

async function saveEntry(data: Record<string, unknown>) {
  saving.value = true
  try {
    if (editingEntry.value) {
      await contentApi.updateEntry(uid.value, editingEntry.value.document_id, { data })
      ElMessage.success('条目已更新')
    } else {
      await contentApi.createEntry(uid.value, {
        data,
      })
      ElMessage.success('条目草稿已创建')
    }
    editorVisible.value = false
    fetchEntries()
  } finally {
    saving.value = false
  }
}

async function publish(entry: ContentEntry) {
  await contentApi.publishEntry(uid.value, entry.document_id)
  ElMessage.success('条目已发布')
  fetchEntries()
}

async function unpublish(entry: ContentEntry) {
  await contentApi.unpublishEntry(uid.value, entry.document_id)
  ElMessage.success('条目已下线')
  fetchEntries()
}

async function removeEntry(entry: ContentEntry) {
  await contentApi.deleteEntry(uid.value, entry.document_id)
  ElMessage.success('条目已删除')
  fetchEntries()
}

async function exportEntries() {
  const res = await contentApi.exportEntries(uid.value)
  const blob = new Blob([res.data.json], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${uid.value}-entries.json`
  a.click()
  URL.revokeObjectURL(url)
}

async function importEntries() {
  if (!importJson.value.trim()) {
    ElMessage.warning('请粘贴 JSON 数据')
    return
  }
  importing.value = true
  try {
    const res = await contentApi.importEntries(uid.value, importJson.value)
    ElMessage.success(`已导入 ${res.data.imported} 条`)
    importVisible.value = false
    importJson.value = ''
    fetchEntries()
  } finally {
    importing.value = false
  }
}

async function showTranslations(entry: ContentEntry) {
  translationSource.value = entry
  newLocale.value = ''
  translationsVisible.value = true
  translationsLoading.value = true
  try {
    translations.value = await queryContentTranslations(uid.value, entry.document_id)
  } finally {
    translationsLoading.value = false
  }
}

function editTranslation(entry: ContentEntry) {
  translationsVisible.value = false
  openEditor(entry)
}

async function createTranslation() {
  const src = translationSource.value
  const locale = newLocale.value.trim()
  if (!src || !locale) {
    ElMessage.warning('请选择目标语言')
    return
  }
  if (locale === src.locale || translations.value.some(t => t.locale === locale)) {
    ElMessage.warning(`语言 ${locale} 的翻译已存在`)
    return
  }
  translating.value = true
  try {
    // Copy the source data as the starting draft; user edits it afterwards.
    await contentApi.createTranslation(uid.value, src.document_id, locale, { data: src.data })
    ElMessage.success(`已创建 ${locale} 翻译草稿`)
    newLocale.value = ''
    translations.value = await queryContentTranslations(uid.value, src.document_id)
    fetchEntries()
  } finally {
    translating.value = false
  }
}

async function load() {
  page.value = 1
  statusFilter.value = ''
  search.value = ''
  await fetchType()
  await fetchEntries()
}

watch(uid, load)
onMounted(load)
</script>

<style lang="scss" scoped>
.entry-page {
  .page-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
    h2 { margin: 0; }
    .header-actions {
      margin-left: auto;
      display: flex;
      gap: 8px;
    }
  }
  .filter-bar {
    display: flex;
    gap: 12px;
    margin-bottom: 12px;
  }
  .pagination {
    margin-top: 16px;
    justify-content: flex-end;
  }
}

.translation-source {
  margin-bottom: 12px;
  color: var(--el-text-color-regular);
}
.translation-create {
  display: flex;
  gap: 8px;
}
.translation-hint {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
