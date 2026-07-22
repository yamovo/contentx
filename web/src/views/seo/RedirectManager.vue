<template>
  <div class="redirect-page">
    <div class="page-header">
      <el-button text @click="$router.back()"><el-icon><ArrowLeft /></el-icon> 返回</el-button>
      <h2>URL 重定向</h2>
      <el-button type="primary" @click="openDialog()"><el-icon><Plus /></el-icon> 添加规则</el-button>
    </div>

    <el-card shadow="never">
      <el-table :data="redirects" v-loading="loading">
        <el-table-column label="来源路径" prop="from_path" min-width="200" />
        <el-table-column label="目标路径" prop="to_path" min-width="200" />
        <el-table-column label="状态码" prop="status_code" width="100" />
        <el-table-column label="访问次数" prop="hit_count" width="100" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'" size="small">{{ row.is_active ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-popconfirm title="确认删除？" @confirm="deleteRedirect(row.id)">
              <template #reference><el-button text size="small" type="danger">删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="添加重定向" width="500px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="来源"><el-input v-model="form.from_path" placeholder="/old-path" /></el-form-item>
        <el-form-item label="目标"><el-input v-model="form.to_path" placeholder="/new-path" /></el-form-item>
        <el-form-item label="状态码">
          <el-select v-model="form.status_code">
            <el-option :value="301" label="301 永久重定向" />
            <el-option :value="302" label="302 临时重定向" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.note" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createRule">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { seoApi, type Redirect } from '@/api'
import { ElMessage } from 'element-plus'

const redirects = ref<Redirect[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const form = reactive({ from_path: '', to_path: '', status_code: 301, note: '' })

async function fetchRedirects() {
  loading.value = true
  try { redirects.value = (await seoApi.listRedirects()).data } catch {}
  finally { loading.value = false }
}

function openDialog() {
  Object.assign(form, { from_path: '', to_path: '', status_code: 301, note: '' })
  dialogVisible.value = true
}

async function createRule() {
  await seoApi.createRedirect(form)
  ElMessage.success('规则已添加')
  dialogVisible.value = false
  fetchRedirects()
}

async function deleteRedirect(id: number) {
  await seoApi.deleteRedirect(id)
  ElMessage.success('规则已删除')
  fetchRedirects()
}

onMounted(fetchRedirects)
</script>

<style lang="scss" scoped>
.redirect-page { .page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; h2 { margin: 0; } } }
</style>
