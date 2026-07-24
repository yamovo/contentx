<template>
  <div class="analytics-page">
    <h2>数据分析</h2>

    <el-row :gutter="16">
      <el-col :span="16">
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span>访问趋势</span>
              <el-radio-group
                v-model="days"
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
            class="chart-lg"
            :option="lineOption"
            autoresize
          />
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="never">
          <template #header>
            <span>设备分布</span>
          </template>
          <v-chart
            class="chart-md"
            :option="pieOption"
            autoresize
          />
        </el-card>
      </el-col>
    </el-row>

    <el-row
      :gutter="16"
      style="margin-top: 16px"
    >
      <el-col :span="12">
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
        </el-card>
      </el-col>
      <el-col :span="12">
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
import { ref, computed, onMounted } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { analyticsApi, articleApi, type Article } from '@/api'

use([CanvasRenderer, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent])

const days = ref(30)
const viewsData = ref<{ date: string; views: number }[]>([])
const devices = ref<any[]>([])
const topArticles = ref<Article[]>([])
const referrers = ref<{ referrer: string; count: number }[]>([])

const lineOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: { type: 'category', data: viewsData.value.map(d => d.date.slice(5)) },
  yAxis: { type: 'value' },
  series: [{
    data: viewsData.value.map(d => d.views), type: 'line', smooth: true,
    areaStyle: { opacity: 0.15 }, itemStyle: { color: '#409eff' },
  }],
}))

const pieOption = computed(() => ({
  tooltip: { trigger: 'item' },
  series: [{
    type: 'pie', radius: ['40%', '70%'], padAngle: 2,
    itemStyle: { borderRadius: 6 },
    label: { formatter: '{b}: {d}%' },
    data: devices.value,
  }],
}))

async function fetchViews() {
  try { viewsData.value = (await analyticsApi.viewsOverTime(days.value)).data } catch {}
}
async function fetchDevices() {
  try {
    const res = await analyticsApi.deviceBreakdown()
    devices.value = (res.data?.devices || []).map((d) => ({ name: d.name, value: d.count }))
  } catch {}
}
async function fetchTop() {
  try {
    const res = await articleApi.list({ sort: 'views', page_size: 10, status: 'published' })
    topArticles.value = res.items
  } catch {}
}
async function fetchReferrers() {
  try { referrers.value = (await analyticsApi.topReferrers()).data } catch {}
}

onMounted(() => { fetchViews(); fetchDevices(); fetchTop(); fetchReferrers() })
</script>

<style lang="scss" scoped>
.analytics-page {
  h2 { margin-bottom: 16px; }
  .card-header { display: flex; justify-content: space-between; align-items: center; }
  .chart-lg { height: 360px; }
  .chart-md { height: 300px; }
  .rank-item {
    display: flex; align-items: center; padding: 10px 0; border-bottom: 1px solid #f0f0f0;
    &:last-child { border-bottom: none; }
    .rank-num { width: 24px; height: 24px; border-radius: 50%; background: #f0f0f0;
      display: flex; align-items: center; justify-content: center; font-size: 12px;
      font-weight: 600; margin-right: 12px; flex-shrink: 0;
      &.top { background: #409eff; color: #fff; } }
    .rank-title { flex: 1; font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .rank-value { font-size: 13px; color: #909399; margin-left: 12px; flex-shrink: 0; }
  }
}
</style>
