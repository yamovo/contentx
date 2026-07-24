<template>
  <div
    ref="pageRef"
    class="dashboard"
  >
    <!-- Stats Cards -->
    <el-row
      :gutter="16"
      class="stats-row"
    >
      <el-col
        v-for="(card, idx) in statCards"
        :key="card.label"
        :xs="12"
        :sm="6"
      >
        <el-card
          shadow="hover"
          class="stat-card"
          :body-style="{ padding: '20px' }"
        >
          <div class="stat-content">
            <div class="stat-info">
              <span class="stat-label">{{ card.label }}</span>
              <span class="stat-value">{{ displayValues[idx] ?? card.value }}</span>
              <span
                class="stat-change"
                :class="card.trend"
              >
                <el-icon><Top v-if="card.trend === 'up'" /><Bottom v-else /></el-icon>
                {{ card.change }}
              </span>
            </div>
            <div
              class="stat-icon"
              :style="{ background: card.color }"
            >
              <el-icon :size="24">
                <component :is="card.icon" />
              </el-icon>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Charts Row -->
    <el-row
      :gutter="16"
      class="chart-row"
    >
      <el-col
        :xs="24"
        :lg="16"
      >
        <el-card
          shadow="hover"
          class="chart-card-left"
        >
          <template #header>
            <div class="card-header">
              <span>访问趋势</span>
              <el-radio-group
                v-model="chartDays"
                size="small"
                @change="fetchViews"
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
          <v-chart
            class="chart"
            :option="viewsChartOption"
            autoresize
          />
        </el-card>
      </el-col>

      <el-col
        :xs="24"
        :lg="8"
      >
        <el-card
          shadow="hover"
          class="chart-card-right"
        >
          <template #header>
            <span>设备分布</span>
          </template>
          <v-chart
            class="chart"
            :option="deviceChartOption"
            autoresize
          />
        </el-card>
      </el-col>
    </el-row>

    <!-- Recent Activity -->
    <el-row :gutter="16">
      <el-col
        :xs="24"
        :lg="12"
      >
        <el-card
          shadow="hover"
          class="activity-card"
        >
          <template #header>
            <div class="card-header">
              <span>最新文章</span>
              <el-button
                text
                type="primary"
                @click="$router.push('/admin/articles')"
              >
                查看全部
              </el-button>
            </div>
          </template>
          <div class="article-list">
            <div
              v-for="article in recentArticles"
              :key="article.id"
              class="article-item"
            >
              <div class="article-info">
                <router-link
                  :to="'/admin/articles/' + article.id + '/edit'"
                  class="article-title"
                >
                  {{ article.title }}
                </router-link>
                <div class="article-meta">
                  <el-tag
                    :type="statusType(article.status)"
                    size="small"
                  >
                    {{ statusLabel(article.status) }}
                  </el-tag>
                  <span>{{ article.author?.display_name }}</span>
                  <span>{{ formatDate(article.created_at, 'MM-DD HH:mm') }}</span>
                </div>
              </div>
              <div class="article-stats">
                <span><el-icon><View /></el-icon> {{ article.view_count }}</span>
                <span><el-icon><ChatDotSquare /></el-icon> {{ article.comment_count }}</span>
              </div>
            </div>
            <el-empty
              v-if="!recentArticles.length"
              description="暂无文章"
              :image-size="80"
            />
          </div>
        </el-card>
      </el-col>

      <el-col
        :xs="24"
        :lg="12"
      >
        <el-card
          shadow="hover"
          class="activity-card"
        >
          <template #header>
            <div class="card-header">
              <span>最新评论</span>
              <el-button
                text
                type="primary"
                @click="$router.push('/admin/comments')"
              >
                查看全部
              </el-button>
            </div>
          </template>
          <div class="comment-list">
            <div
              v-for="comment in recentComments"
              :key="comment.id"
              class="comment-item"
            >
              <el-avatar :size="36">
                {{ (comment.author_name || 'U')[0] }}
              </el-avatar>
              <div class="comment-content">
                <div class="comment-header">
                  <strong>{{ comment.author_name || comment.user?.display_name || '匿名' }}</strong>
                  <el-tag
                    :type="commentStatusType(comment.status)"
                    size="small"
                  >
                    {{ comment.status }}
                  </el-tag>
                </div>
                <p class="comment-text">
                  {{ truncate(comment.content, 80) }}
                </p>
                <span class="comment-time">{{ formatDate(comment.created_at, 'MM-DD HH:mm') }}</span>
              </div>
            </div>
            <el-empty
              v-if="!recentComments.length"
              description="暂无评论"
              :image-size="80"
            />
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive, nextTick } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, TitleComponent } from 'echarts/components'
import { analyticsApi, type DashboardStats, type Article, type Comment } from '@/api'
import { formatDate } from '@/utils'
import { animate } from 'animejs'
import { stagger } from 'animejs/utils'

use([CanvasRenderer, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent])

const stats = ref<DashboardStats>({
  total_articles: 0, published_articles: 0, total_comments: 0,
  pending_comments: 0, total_users: 0, total_media: 0,
  views_today: 0, views_this_week: 0, views_this_month: 0, total_views: 0,
})
const recentArticles = ref<Article[]>([])
const recentComments = ref<Comment[]>([])
const chartDays = ref(30)
const viewsData = ref<{ date: string; views: number }[]>([])
const deviceData = ref<any[]>([])
const pageRef = ref<HTMLElement>()

const displayValues = reactive<(number | string)[]>([0, 0, 0, 0])

const statCards = computed(() => [
  { label: '文章总数', value: stats.value.total_articles, icon: 'Document', color: '#409eff', change: '+12', trend: 'up' },
  { label: '今日访问', value: stats.value.views_today, icon: 'View', color: '#67c23a', change: '+8%', trend: 'up' },
  { label: '待审评论', value: stats.value.pending_comments, icon: 'ChatDotSquare', color: '#e6a23c', change: '', trend: '' },
  { label: '用户总数', value: stats.value.total_users, icon: 'User', color: '#909399', change: '+3', trend: 'up' },
])

const viewsChartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: {
    type: 'category',
    data: viewsData.value.map(d => d.date),
    axisLabel: { formatter: (v: string) => v.slice(5) },
  },
  yAxis: { type: 'value' },
  series: [{
    data: viewsData.value.map(d => d.views),
    type: 'line',
    smooth: true,
    areaStyle: { opacity: 0.15 },
    lineStyle: { width: 2 },
    itemStyle: { color: '#409eff' },
  }],
}))

const deviceChartOption = computed(() => ({
  tooltip: { trigger: 'item' },
  series: [{
    type: 'pie',
    radius: ['40%', '70%'],
    avoidLabelOverlap: false,
    padAngle: 2,
    itemStyle: { borderRadius: 6 },
    label: { show: true, formatter: '{b}: {d}%' },
    data: deviceData.value,
  }],
}))

function statusType(s: string) {
  return s === 'published' ? 'success' : s === 'draft' ? 'info' : s === 'pending' ? 'warning' : 'danger'
}
function statusLabel(s: string) {
  return { published: '已发布', draft: '草稿', pending: '待审', scheduled: '定时', trash: '回收站' }[s] || s
}
function commentStatusType(s: string) {
  return s === 'approved' ? 'success' : s === 'pending' ? 'warning' : 'danger'
}
function truncate(s: string, n: number) {
  return s?.length > n ? s.slice(0, n) + '…' : s
}


function animateCounters() {
  [0, 1, 2, 3].forEach(idx => {
    const card = statCards.value[idx]
    if (!card || card.value === 0) return
    const proxy = { v: 0 }
    animate(proxy, {
      v: card.value,
      duration: 1200,
      delay: idx * 150,
      ease: 'outQuint',
      onUpdate() {
        displayValues[idx] = Math.round(proxy.v)
      },
    })
  })
}

function animateEntrance() {
  animate('.stat-card', {
    opacity: { from: 0 },
    translateY: { from: 24 },
    scale: { from: 0.95 },
    duration: 600,
    delay: stagger(100),
    ease: 'outQuint',
  })

  animate('.chart-card-left, .chart-card-right', {
    opacity: { from: 0 },
    translateY: { from: 20 },
    duration: 700,
    delay: stagger(120, { start: 300 }),
    ease: 'outQuint',
  })

  animate('.activity-card', {
    opacity: { from: 0 },
    translateY: { from: 20 },
    duration: 700,
    delay: stagger(100, { start: 500 }),
    ease: 'outQuint',
  })
}

async function fetchDashboard() {
  try {
    const res = await analyticsApi.dashboard()
    stats.value = res.stats
    recentArticles.value = res.recent_articles
    recentComments.value = res.recent_comments
    await nextTick()
    animateCounters()
  } catch {
    // ignore
  }
}

async function fetchViews() {
  try {
    const res = await analyticsApi.viewsOverTime(chartDays.value)
    viewsData.value = res.data
  } catch {
    viewsData.value = []
  }
}

async function fetchDevices() {
  try {
    const res = await analyticsApi.deviceBreakdown()
    deviceData.value = (res.data?.devices || []).map((d) => ({
      name: d.name, value: d.count,
    }))
  } catch {
    deviceData.value = [
      { name: 'Desktop', value: 60 },
      { name: 'Mobile', value: 30 },
      { name: 'Tablet', value: 10 },
    ]
  }
}

onMounted(() => {
  animateEntrance()
  fetchDashboard()
  fetchViews()
  fetchDevices()
})
</script>

<style lang="scss" scoped>
.dashboard {
  .stats-row { margin-bottom: 16px; }

  .stat-card {
    .stat-content {
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .stat-info {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .stat-label { font-size: 13px; color: #909399; }
    .stat-value { font-size: 28px; font-weight: 600; color: #303133; }
    .stat-change {
      font-size: 12px;
      &.up { color: #67c23a; }
      &.down { color: #f56c6c; }
    }
    .stat-icon {
      width: 56px; height: 56px; border-radius: 12px;
      display: flex; align-items: center; justify-content: center;
      color: #fff;
    }
  }

  .chart-row { margin-bottom: 16px; }
  .chart { height: 320px; }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .article-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 0;
    border-bottom: 1px solid #f0f0f0;
    transition: background 0.2s;
    &:hover { background: #fafafa; border-radius: 6px; }
    &:last-child { border-bottom: none; }
  }
  .article-title {
    font-size: 14px; color: #303133; text-decoration: none;
    &:hover { color: #409eff; }
  }
  .article-meta {
    display: flex; gap: 8px; align-items: center; margin-top: 4px;
    font-size: 12px; color: #909399;
  }
  .article-stats {
    display: flex; gap: 12px; font-size: 12px; color: #909399;
    span { display: flex; align-items: center; gap: 2px; }
  }

  .comment-item {
    display: flex; gap: 12px; padding: 12px 0;
    border-bottom: 1px solid #f0f0f0;
    transition: background 0.2s;
    &:hover { background: #fafafa; border-radius: 6px; }
    &:last-child { border-bottom: none; }
  }
  .comment-content { flex: 1; }
  .comment-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
  .comment-text { font-size: 13px; color: #606266; margin: 4px 0; }
  .comment-time { font-size: 12px; color: #c0c4cc; }
}
</style>
