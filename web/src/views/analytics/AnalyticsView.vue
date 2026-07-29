<template>
  <div class="analytics-page">
    <div class="page-header">
      <div>
        <h2>数据分析</h2>
        <p>访问趋势、设备分布与内容表现均来自真实统计接口。</p>
      </div>
      <el-button
        :loading="loading"
        @click="loadAll"
      >
        刷新
      </el-button>
    </div>

    <el-alert
      v-if="errorMessage"
      type="error"
      :closable="false"
      show-icon
      class="error-alert"
      :title="errorMessage"
    >
      <el-button
        text
        type="primary"
        @click="loadAll"
      >
        重试
      </el-button>
    </el-alert>

    <el-row :gutter="16">
      <el-col
        :xs="24"
        :lg="16"
      >
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span>访问趋势</span>
              <el-radio-group
                v-model="days"
                size="small"
              >
                <el-radio-button :value="7">
                  7天
                </el-radio-button>
                <el-radio-button :value="30">
                  30天
                </el-radio-button>
                <el-radio-button :value="90">
                  90天
                </el-radio-button>
              </el-radio-group>
            </div>
          </template>
          <SimpleLineChart
            :data="lineData"
            title="访问趋势"
          />
        </el-card>
      </el-col>
      <el-col
        :xs="24"
        :lg="8"
      >
        <el-card shadow="never">
          <template #header>
            <span>设备分布</span>
          </template>
          <SimpleDonutChart
            :data="devices"
            title="设备分布"
          />
        </el-card>
      </el-col>
    </el-row>

    <el-row
      :gutter="16"
      style="margin-top: 16px"
    >
      <el-col
        :xs="24"
        :lg="12"
      >
        <el-card shadow="never">
          <template #header>
            <span>热门文章</span>
          </template>
          <div
            v-for="(a, i) in topArticles"
            :key="a.id"
            class="rank-item"
          >
            <span
              class="rank-num"
              :class="{ top: i < 3 }"
            >{{ i + 1 }}</span>
            <span class="rank-title">{{ a.title }}</span>
            <span class="rank-value">{{ a.view_count }} 次</span>
          </div>
          <el-empty
            v-if="!topArticles.length"
            description="暂无热门内容数据"
            :image-size="60"
          />
        </el-card>
      </el-col>
      <el-col
        :xs="24"
        :lg="12"
      >
        <el-card shadow="never">
          <template #header>
            <span>来源站点</span>
          </template>
          <div
            v-for="(r, i) in referrers"
            :key="i"
            class="rank-item"
          >
            <span class="rank-num">{{ i + 1 }}</span>
            <span class="rank-title">{{ r.referrer }}</span>
            <span class="rank-value">{{ r.count }} 次</span>
          </div>
          <el-empty
            v-if="!referrers.length"
            description="暂无数据"
            :image-size="60"
          />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { articleApi, type Article } from '@/api'
import SimpleLineChart from '@/features/analytics/SimpleLineChart.vue'
import SimpleDonutChart from '@/features/analytics/SimpleDonutChart.vue'
import { useViewsOverTimeQuery, useDeviceBreakdownQuery, useTopReferrersQuery } from '@/features/analytics/use-dashboard-query'

const days = ref(30)

// Passing the ref keeps the views query reactive to the day-range selector.
const { data: viewsData, refetch: refetchViews } = useViewsOverTimeQuery(days)
const { data: devicesData, refetch: refetchDevices } = useDeviceBreakdownQuery()
const { data: referrersData, refetch: refetchReferrers } = useTopReferrersQuery()

// Top articles still uses manual fetch since no dedicated composable exists.
const topArticles = ref<Article[]>([])
const loading = ref(false)
const errorMessage = ref('')

const lineData = computed(() => (viewsData.value?.data ?? []).map((item: { date: string; views: number }) => ({
  label: item.date,
  value: item.views,
})))

const devices = computed(() =>
  (devicesData.value?.data?.devices ?? []).map((d: { name: string; count: number }) => ({ name: d.name, value: d.count })),
)

const referrers = computed(() => referrersData.value?.data ?? [])

async function fetchTop() {
  const res = await articleApi.list({ sort: 'views', page_size: 10, status: 'published' })
  topArticles.value = res.items || []
}

async function loadAll() {
  loading.value = true
  errorMessage.value = ''
  const results = await Promise.allSettled([
    refetchViews(),
    refetchDevices(),
    refetchReferrers(),
    fetchTop(),
  ])
  const failed = results.filter(result => result.status === 'rejected').length
  if (failed) {
    errorMessage.value = failed === results.length
      ? '分析数据加载失败，请检查网络后重试。'
      : `有 ${failed} 组分析数据暂时无法加载，其余数据仍可查看。`
  }
  loading.value = false
}

// Initial load for top articles.
fetchTop()
</script>

<style lang="scss" scoped>
.analytics-page {
  .page-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 16px;
    h2 { margin: 0; }
    p { margin: 6px 0 0; color: var(--app-text-muted, #6b7280); font-size: 13px; }
  }
  .error-alert { margin-bottom: 16px; }
  .card-header { display: flex; justify-content: space-between; align-items: center; }
  .rank-item {
    display: flex; align-items: center; padding: 10px 0; border-bottom: 1px solid var(--app-border-subtle, #e8eaed);
    &:last-child { border-bottom: none; }
    .rank-num { width: 24px; height: 24px; border-radius: 50%; background: var(--app-surface-muted, #f3f4f6);
      display: flex; align-items: center; justify-content: center; font-size: 12px;
      font-weight: 600; margin-right: 12px; flex-shrink: 0;
      &.top { background: var(--app-color-primary, #2563eb); color: #fff; } }
    .rank-title { flex: 1; font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .rank-value { font-size: 13px; color: var(--app-text-muted, #6b7280); margin-left: 12px; flex-shrink: 0; }
  }
}

@media (max-width: 1199px) {
  .analytics-page :deep(.el-col) {
    margin-bottom: 16px;
  }
}
</style>
