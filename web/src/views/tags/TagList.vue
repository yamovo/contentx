<template>
  <div class="tag-page">
    <div class="page-header">
      <h2>标签管理</h2>
      <div class="header-actions">
        <el-input
          v-model="search"
          placeholder="搜索标签..."
          :prefix-icon="Search"
          clearable
          style="width: 200px; margin-right: 12px"
          @input="fetchTags"
        />
        <el-button
          type="primary"
          @click="openDialog()"
        >
          <el-icon><Plus /></el-icon> 新建标签
        </el-button>
      </div>
    </div>

    <el-card shadow="never">
      <div class="tag-cloud">
        <div
          v-for="tag in tags"
          :key="tag.id"
          class="tag-card"
        >
          <div class="tag-info">
            <el-tag
              :color="tag.color"
              effect="dark"
              size="large"
              class="tag-name"
            >
              {{ tag.name }}
            </el-tag>
            <span class="tag-slug">/{{ tag.slug }}</span>
            <span class="tag-count">{{ tag.count }} 篇文章</span>
          </div>
          <div class="tag-actions">
            <el-button
              text
              size="small"
              @click="openDialog(tag)"
            >
              编辑
            </el-button>
            <el-popconfirm
              title="确认删除？"
              @confirm="deleteTag(tag.id)"
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
          </div>
        </div>
      </div>
      <el-empty
        v-if="!tags.length"
        description="暂无标签"
      />
    </el-card>

    <!-- Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑标签' : '新建标签'"
      width="400px"
    >
      <el-form
        :model="form"
        label-width="60px"
      >
        <el-form-item
          label="名称"
          required
        >
          <el-input
            v-model="form.name"
            placeholder="标签名称"
          />
        </el-form-item>
        <el-form-item label="别名">
          <el-input
            v-model="form.slug"
            placeholder="URL 别名"
          />
        </el-form-item>
        <el-form-item label="颜色">
          <el-color-picker v-model="form.color" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          @click="saveTag"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { tagApi, type Tag } from '@/api'
import { ElMessage } from 'element-plus'
import { getApiError } from '@/utils'

const tags = ref<Tag[]>([])
const search = ref('')
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive({ name: '', slug: '', color: '#409eff' })

async function fetchTags() {
  try {
    const res = await tagApi.list({ search: search.value })
    tags.value = res.data
  } catch { tags.value = [] }
}

function openDialog(tag?: Tag) {
  if (tag) {
    editingId.value = tag.id
    Object.assign(form, { name: tag.name, slug: tag.slug, color: tag.color || '#409eff' })
  } else {
    editingId.value = null
    Object.assign(form, { name: '', slug: '', color: '#409eff' })
  }
  dialogVisible.value = true
}

async function saveTag() {
  if (!form.name.trim()) { ElMessage.warning('请输入标签名称'); return }
  try {
    if (editingId.value) {
      await tagApi.update(editingId.value, form)
      ElMessage.success('标签已更新')
    } else {
      await tagApi.create(form)
      ElMessage.success('标签已创建')
    }
    dialogVisible.value = false
    fetchTags()
  } catch (err) {
    ElMessage.error(getApiError(err, '保存失败'))
  }
}

async function deleteTag(id: number) {
  try {
    await tagApi.delete(id)
    ElMessage.success('标签已删除')
    fetchTags()
  } catch { ElMessage.error('删除失败') }
}

onMounted(fetchTags)
</script>

<style lang="scss" scoped>
.tag-page {
  .page-header {
    display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;
    h2 { margin: 0; font-size: 20px; }
    .header-actions { display: flex; align-items: center; }
  }
  .tag-cloud { display: flex; flex-wrap: wrap; gap: 12px; }
  .tag-card {
    display: flex; align-items: center; justify-content: space-between;
    padding: 12px 16px; border: 1px solid #ebeef5; border-radius: 8px;
    min-width: 280px; flex: 1; max-width: 400px;
    .tag-info { display: flex; align-items: center; gap: 8px; }
    .tag-slug { font-size: 12px; color: #909399; }
    .tag-count { font-size: 12px; color: #c0c4cc; }
    .tag-actions { display: flex; gap: 4px; }
  }
}
</style>
