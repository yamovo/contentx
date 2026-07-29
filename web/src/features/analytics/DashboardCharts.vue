<template>
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
              v-model="daysModel"
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
        <div
          v-if="!viewsData.length"
          class="chart-empty"
        >
          暂无数据
        </div>
        <v-chart
          v-else
          class="chart"
          :option="viewsOption"
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
        <div
          v-if="!deviceData.length"
          class="chart-empty"
        >
          暂无数据
        </div>
        <v-chart
          v-else
          class="chart"
          :option="deviceOption"
          autoresize
        />
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, TitleComponent } from 'echarts/components'

use([CanvasRenderer, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent])

const props = defineProps<{
  viewsData: { date: string; views: number }[]
  deviceData: { name: string; value: number }[]
  days: number
}>()

const emit = defineEmits<{
  'update:days': [value: number]
}>()

const daysModel = computed({
  get: () => props.days,
  set: (v: number) => emit('update:days', v),
})

const viewsOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: {
    type: 'category',
    data: props.viewsData.map(d => d.date),
    axisLabel: { formatter: (v: string) => v.slice(5) },
  },
  yAxis: { type: 'value' },
  series: [{
    data: props.viewsData.map(d => d.views),
    type: 'line',
    smooth: true,
    areaStyle: { opacity: 0.15 },
    lineStyle: { width: 2 },
    itemStyle: { color: '#409eff' },
  }],
}))

const deviceOption = computed(() => ({
  tooltip: { trigger: 'item' },
  series: [{
    type: 'pie',
    radius: ['40%', '70%'],
    avoidLabelOverlap: false,
    padAngle: 2,
    itemStyle: { borderRadius: 6 },
    label: { show: true, formatter: '{b}: {d}%' },
    data: props.deviceData,
  }],
}))
</script>

<style scoped>
.chart-row { margin-bottom: 16px; }
.chart { height: 320px; }
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.chart-card-left,
.chart-card-right {
  /* inherit parent card styling */
}
.chart-empty {
  height: 320px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #909399;
  font-size: 14px;
}
</style>
