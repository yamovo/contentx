<template>
  <div class="category-page">
    <div class="page-header">
      <h2>分类管理</h2>
      <el-button
        type="primary"
        @click="openDialog()"
      >
        <el-icon><Plus /></el-icon> 新建分类
      </el-button>
    </div>

    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="categories"
        row-key="id"
        default-expand-all
      >
        <el-table-column
          label="名称"
          min-width="250"
        >
          <template #default="{ row }">
            <div class="cat-name">
              <span
                class="color-dot"
                :style="{ background: row.color || '#409eff' }"
              />
              <strong>{{ row.name }}</strong>
              <el-tag
                v-if="!row.is_active"
                type="danger"
                size="small"
              >
                已禁用
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          label="别名"
          prop="slug"
          width="150"
        />
        <el-table-column
          label="文章数"
          prop="post_count"
          width="80"
          align="center"
        />
        <el-table-column
          label="排序"
          prop="sort_order"
          width="80"
          align="center"
        />
        <el-table-column
          label="操作"
          width="200"
        >
          <template #default="{ row }">
            <el-button
              text
              size="small"
              @click="openDialog(row as Category)"
            >
              编辑
            </el-button>
            <el-popconfirm
              title="确认删除？文章将移至未分类"
              @confirm="deleteCategory(row.id)"
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
    </el-card>

    <!-- Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑分类' : '新建分类'"
      width="500px"
    >
      <el-form
        :model="form"
        label-width="80px"
      >
        <el-form-item
          label="名称"
          required
        >
          <el-input
            v-model="form.name"
            placeholder="分类名称"
          />
        </el-form-item>
        <el-form-item label="别名">
          <el-input
            v-model="form.slug"
            placeholder="URL 别名（留空自动生成）"
          />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
          />
        </el-form-item>
        <el-form-item label="父分类">
          <el-tree-select
            v-model="form.parent_id"
            :data="categoryTree"
            :props="treeSelectProps"
            placeholder="顶级分类"
            check-strictly
            clearable
          />
        </el-form-item>
        <el-form-item label="颜色">
          <el-color-picker v-model="form.color" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number
            v-model="form.sort_order"
            :min="0"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch
            v-model="form.is_active"
            active-text="启用"
            inactive-text="禁用"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="saveCategory"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { categoryApi, type Category } from '@/api'
import { ElMessage } from 'element-plus'
import { buildTree, getApiError } from '@/utils'

const categories = ref<Category[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)

const treeSelectProps = { label: 'name', value: 'id', children: 'children' }

const form = reactive({
  name: '', slug: '', description: '', parent_id: null as number | null,
  color: '#409eff', sort_order: 0, is_active: true,
})

const categoryTree = computed(() => buildTree(categories.value))

async function fetchCategories() {
  loading.value = true
  try {
    const res = await categoryApi.list({ all: 'true' })
    categories.value = res.data
  } catch {
    categories.value = []
  } finally {
    loading.value = false
  }
}

function openDialog(cat?: Category) {
  if (cat) {
    editingId.value = cat.id
    Object.assign(form, {
      name: cat.name, slug: cat.slug, description: cat.description,
      parent_id: cat.parent_id, color: cat.color || '#409eff',
      sort_order: cat.sort_order, is_active: cat.is_active,
    })
  } else {
    editingId.value = null
    Object.assign(form, {
      name: '', slug: '', description: '', parent_id: null,
      color: '#409eff', sort_order: 0, is_active: true,
    })
  }
  dialogVisible.value = true
}

async function saveCategory() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入分类名称')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await categoryApi.update(editingId.value, form)
      ElMessage.success('分类已更新')
    } else {
      await categoryApi.create(form)
      ElMessage.success('分类已创建')
    }
    dialogVisible.value = false
    fetchCategories()
  } catch (err) {
    ElMessage.error(getApiError(err, '保存失败'))
  } finally {
    saving.value = false
  }
}

async function deleteCategory(id: number) {
  try {
    await categoryApi.delete(id)
    ElMessage.success('分类已删除')
    fetchCategories()
  } catch {
    ElMessage.error('删除失败')
  }
}

onMounted(fetchCategories)
</script>

<style lang="scss" scoped>
.category-page {
  .page-header {
    display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;
    h2 { margin: 0; font-size: 20px; }
  }
  .cat-name {
    display: flex; align-items: center; gap: 8px;
    .color-dot {
      width: 12px; height: 12px; border-radius: 50%; display: inline-block;
    }
  }
}
</style>
