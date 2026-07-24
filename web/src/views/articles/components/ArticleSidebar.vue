<template>
  <div>
    <!-- Publish Settings -->
    <el-card
      shadow="never"
      class="section-card"
    >
      <template #header>
        <span>发布设置</span>
      </template>
      <el-form
        label-position="top"
        size="small"
      >
        <el-form-item label="状态">
          <el-select
            v-model="form.status"
            style="width: 100%"
          >
            <el-option
              label="草稿"
              value="draft"
            />
            <el-option
              label="已发布"
              value="published"
            />
            <el-option
              label="待审核"
              value="pending"
            />
            <el-option
              label="定时发布"
              value="scheduled"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="可见性">
          <el-select
            v-model="form.visibility"
            style="width: 100%"
          >
            <el-option
              label="公开"
              value="public"
            />
            <el-option
              label="私密"
              value="private"
            />
            <el-option
              label="密码保护"
              value="password"
            />
          </el-select>
        </el-form-item>
        <el-form-item
          v-if="form.visibility === 'password'"
          label="访问密码"
        >
          <el-input
            v-model="form.password"
            show-password
          />
        </el-form-item>
        <el-form-item
          v-if="form.status === 'scheduled'"
          label="定时发布"
        >
          <el-date-picker
            v-model="form.scheduled_at"
            type="datetime"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="form.allow_comment">
            允许评论
          </el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="form.is_pinned">
            置顶文章
          </el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="form.is_featured">
            设为精选
          </el-checkbox>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Category -->
    <el-card
      shadow="never"
      class="section-card"
    >
      <template #header>
        <div class="card-header-row">
          <span>分类</span>
          <el-button
            text
            size="small"
            type="primary"
          >
            + 新建
          </el-button>
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
    <el-card
      shadow="never"
      class="section-card"
    >
      <template #header>
        <span>标签</span>
      </template>
      <el-select
        v-model="form.tag_ids"
        multiple
        filterable
        allow-create
        default-first-option
        placeholder="选择或创建标签"
        style="width: 100%"
        @create="(name: string) => $emit('create-tag', name)"
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
    <el-card
      shadow="never"
      class="section-card"
    >
      <template #header>
        <span>特色图片</span>
      </template>
      <div
        v-if="form.featured_image"
        class="featured-preview"
      >
        <img
          :src="form.featured_image"
          alt="Featured"
        >
        <el-button
          text
          type="danger"
          @click="form.featured_image = ''"
        >
          移除
        </el-button>
      </div>
      <el-upload
        v-else
        action="/api/v1/media/upload"
        :headers="uploadHeaders"
        :on-success="onUploadSuccess"
        :show-file-list="false"
        accept="image/*"
      >
        <el-button>上传图片</el-button>
      </el-upload>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import type { Category, Tag } from '@/api'

defineProps<{
  form: {
    status: string
    visibility: string
    password: string
    scheduled_at: Date | null
    allow_comment: boolean
    is_pinned: boolean
    is_featured: boolean
    category_id: number | null
    tag_ids: number[]
    featured_image: string
  }
  allTags: Tag[]
  categoryTree: (Category & { children: Category[] })[]
  treeSelectProps: { label: string; value: string; children: string }
  uploadHeaders: Record<string, string>
}>()

const emit = defineEmits<{
  'create-tag': [name: string]
  'upload-success': [res: unknown]
}>()

function onUploadSuccess(res: unknown) {
  emit('upload-success', res)
}
</script>

<style lang="scss" scoped>
.section-card {
  margin-bottom: 16px;
}

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
</style>
