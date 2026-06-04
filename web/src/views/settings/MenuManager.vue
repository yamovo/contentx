<template>
  <div class="menu-page">
    <div class="page-header">
      <h2>导航菜单</h2>
      <el-button type="primary" @click="createMenu"><el-icon><Plus /></el-icon> 新建菜单</el-button>
    </div>

    <el-row :gutter="16">
      <el-col :span="8">
        <el-card shadow="never">
          <template #header><span>菜单列表</span></template>
          <div v-for="m in menus" :key="m.id" class="menu-item" :class="{ active: selectedMenu?.id === m.id }" @click="selectMenu(m)">
            <span>{{ m.name }}</span>
            <span class="menu-loc">{{ m.locations }}</span>
          </div>
          <el-empty v-if="!menus.length" description="暂无菜单" :image-size="60" />
        </el-card>
      </el-col>

      <el-col :span="16">
        <el-card v-if="selectedMenu" shadow="never">
          <template #header>
            <div class="card-header">
              <span>{{ selectedMenu.name }} — 菜单项</span>
              <el-button size="small" type="primary" @click="addItem">添加项目</el-button>
            </div>
          </template>
          <div v-for="item in selectedMenu.items" :key="item.id" class="menu-entry">
            <div class="entry-info">
              <el-icon><Rank /></el-icon>
              <span class="entry-title">{{ item.title }}</span>
              <span class="entry-url">{{ item.url }}</span>
              <el-tag v-if="!item.is_active" size="small" type="info">禁用</el-tag>
            </div>
            <div class="entry-actions">
              <el-button text size="small" @click="editItem(item)">编辑</el-button>
              <el-button text size="small" type="danger" @click="deleteItem(item.id)">删除</el-button>
            </div>
          </div>
          <el-empty v-if="!selectedMenu.items?.length" description="暂无菜单项" :image-size="60" />
        </el-card>
        <el-card v-else shadow="never">
          <el-empty description="选择一个菜单" />
        </el-card>
      </el-col>
    </el-row>

    <el-dialog v-model="itemDialog" :title="editingItemId ? '编辑菜单项' : '添加菜单项'" width="500px">
      <el-form :model="itemForm" label-width="80px">
        <el-form-item label="标题" required><el-input v-model="itemForm.title" /></el-form-item>
        <el-form-item label="URL"><el-input v-model="itemForm.url" /></el-form-item>
        <el-form-item label="打开方式">
          <el-select v-model="itemForm.target">
            <el-option label="当前窗口" value="_self" />
            <el-option label="新窗口" value="_blank" />
          </el-select>
        </el-form-item>
        <el-form-item label="图标"><el-input v-model="itemForm.icon" placeholder="图标类名" /></el-form-item>
        <el-form-item label="CSS 类"><el-input v-model="itemForm.css_class" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="itemDialog = false">取消</el-button>
        <el-button type="primary" @click="saveItem">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { menuApi, type Menu, type MenuItem } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'

const menus = ref<Menu[]>([])
const selectedMenu = ref<Menu | null>(null)
const itemDialog = ref(false)
const editingItemId = ref<number | null>(null)
const itemForm = reactive({ title: '', url: '', target: '_self', icon: '', css_class: '' })

async function fetchMenus() {
  try { menus.value = (await menuApi.list()).data } catch {}
}

function selectMenu(m: Menu) { selectedMenu.value = m }

async function createMenu() {
  const { value } = await ElMessageBox.prompt('输入菜单名称', '新建菜单')
  await menuApi.create({ name: value, slug: value.toLowerCase().replace(/\s+/g, '-') })
  ElMessage.success('菜单已创建')
  fetchMenus()
}

function addItem() {
  editingItemId.value = null
  Object.assign(itemForm, { title: '', url: '', target: '_self', icon: '', css_class: '' })
  itemDialog.value = true
}

function editItem(item: MenuItem) {
  editingItemId.value = item.id
  Object.assign(itemForm, { title: item.title, url: item.url, target: item.target, icon: item.icon, css_class: item.css_class })
  itemDialog.value = true
}

async function saveItem() {
  if (!selectedMenu.value) return
  if (editingItemId.value) {
    await menuApi.updateItem(selectedMenu.value.id, editingItemId.value, itemForm)
  } else {
    await menuApi.addItem(selectedMenu.value.id, itemForm)
  }
  ElMessage.success('已保存')
  itemDialog.value = false
  // Refresh menu.
  const res = await menuApi.get(selectedMenu.value.id)
  selectedMenu.value = res.data
  fetchMenus()
}

async function deleteItem(id: number) {
  if (!selectedMenu.value) return
  await ElMessageBox.confirm('确认删除？')
  await menuApi.deleteItem(selectedMenu.value.id, id)
  const res = await menuApi.get(selectedMenu.value.id)
  selectedMenu.value = res.data
  ElMessage.success('已删除')
}

onMounted(fetchMenus)
</script>

<style lang="scss" scoped>
.menu-page {
  .page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; h2 { margin: 0; } }
  .menu-item {
    padding: 12px; cursor: pointer; border-radius: 6px; margin-bottom: 6px;
    display: flex; justify-content: space-between; align-items: center;
    &:hover { background: #f5f7fa; }
    &.active { background: #ecf5ff; color: #409eff; }
    .menu-loc { font-size: 12px; color: #909399; }
  }
  .card-header { display: flex; justify-content: space-between; align-items: center; }
  .menu-entry {
    display: flex; justify-content: space-between; align-items: center;
    padding: 10px 0; border-bottom: 1px solid #f0f0f0;
    .entry-info { display: flex; align-items: center; gap: 8px; }
    .entry-title { font-weight: 500; }
    .entry-url { font-size: 12px; color: #909399; }
  }
}
</style>
