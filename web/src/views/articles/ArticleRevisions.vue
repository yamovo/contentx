<template>
  <div class="revisions-page">
    <div class="page-header">
      <el-button
        text
        @click="$router.back()"
      >
        <el-icon><ArrowLeft /></el-icon> 返回
      </el-button>
      <h2>版本历史 — {{ articleTitle }}</h2>
    </div>

    <el-card shadow="never">
      <el-timeline>
        <el-timeline-item
          v-for="rev in revisions"
          :key="rev.id"
          :timestamp="formatDate(rev.created_at, 'YYYY-MM-DD HH:mm:ss')"
          placement="top"
          :type="rev.version === currentVersion ? 'primary' : ''"
        >
          <el-card
            shadow="hover"
            class="revision-card"
          >
            <div class="revision-header">
              <span class="version-badge">v{{ rev.version }}</span>
              <span
                v-if="rev.version === currentVersion"
                class="current-tag"
              >当前版本</span>
              <span class="editor">{{ rev.editor?.display_name || 'Unknown' }}</span>
              <span class="note">{{ rev.note }}</span>
            </div>
            <div class="revision-meta">
              标题: {{ rev.title }} | 字数: {{ rev.content?.length || 0 }}
            </div>
            <div
              v-if="rev.version !== currentVersion"
              class="revision-actions"
            >
              <el-button
                size="small"
                type="primary"
                @click="restoreRevision(rev.id)"
              >
                恢复此版本
              </el-button>
              <el-button
                size="small"
                @click="viewDiff(rev)"
              >
                查看差异
              </el-button>
            </div>
          </el-card>
        </el-timeline-item>
      </el-timeline>

      <el-empty
        v-if="!revisions.length"
        description="暂无版本历史"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { articleApi, type Revision } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatDate } from '@/utils'

const route = useRoute()
const articleId = Number(route.params.id)
const articleTitle = ref('')
const revisions = ref<Revision[]>([])
const currentVersion = ref(0)

async function fetchRevisions() {
  try {
    const [articleRes, revRes] = await Promise.all([
      articleApi.get(articleId),
      articleApi.revisions(articleId),
    ])
    articleTitle.value = articleRes.data.title
    revisions.value = revRes.data
    if (revisions.value.length > 0) {
      currentVersion.value = Math.max(...revisions.value.map(r => r.version))
    }
  } catch {
    ElMessage.error('获取版本历史失败')
  }
}

async function restoreRevision(revisionId: number) {
  await ElMessageBox.confirm('确认恢复到此版本？当前内容将被保存为新版本。', '确认恢复')
  try {
    await articleApi.restoreRevision(articleId, revisionId)
    ElMessage.success('版本已恢复')
    fetchRevisions()
  } catch {
    ElMessage.error('恢复失败')
  }
}

function viewDiff(_rev: Revision) {
  ElMessage.info('差异查看功能开发中')
}



onMounted(fetchRevisions)
</script>

<style lang="scss" scoped>
.revisions-page {
  .page-header {
    margin-bottom: 16px;
    h2 { margin: 8px 0 0; font-size: 18px; }
  }
  .revision-card {
    .revision-header {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;
    }
    .version-badge {
      background: #409eff;
      color: #fff;
      padding: 2px 8px;
      border-radius: 10px;
      font-size: 12px;
      font-weight: 600;
    }
    .current-tag {
      background: #67c23a;
      color: #fff;
      padding: 2px 8px;
      border-radius: 10px;
      font-size: 11px;
    }
    .editor { font-weight: 500; }
    .note { color: #909399; font-size: 13px; }
    .revision-meta { font-size: 13px; color: #606266; margin-bottom: 8px; }
    .revision-actions { margin-top: 8px; }
  }
}
</style>
