<template>
  <div
    class="line-chart"
    role="img"
    :aria-label="ariaLabel"
  >
    <svg
      v-if="points.length > 1"
      viewBox="0 0 720 260"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      <g class="grid">
        <line
          v-for="line in 5"
          :key="line"
          x1="44"
          x2="708"
          :y1="20 + (line - 1) * 52"
          :y2="20 + (line - 1) * 52"
        />
      </g>
      <path
        class="area"
        :d="areaPath"
      />
      <path
        class="line"
        :d="linePath"
      />
      <circle
        v-for="point in points"
        :key="point.label"
        :cx="point.x"
        :cy="point.y"
        r="4"
      >
        <title>{{ point.label }}：{{ point.value }}</title>
      </circle>
    </svg>
    <div
      v-else
      class="empty"
    >
      暂无趋势数据
    </div>
    <div
      v-if="points.length > 1"
      class="axis-labels"
      aria-hidden="true"
    >
      <span>{{ points[0]?.label }}</span>
      <span>{{ points[Math.floor(points.length / 2)]?.label }}</span>
      <span>{{ points[points.length - 1]?.label }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Datum {
  label: string
  value: number
}

const props = defineProps<{
  data: Datum[]
  title?: string
}>()

const points = computed(() => {
  if (!props.data.length) return []
  const values = props.data.map(item => Number(item.value) || 0)
  const max = Math.max(...values, 1)
  const min = Math.min(...values, 0)
  const range = Math.max(max - min, 1)
  const width = 664
  return props.data.map((item, index) => ({
    ...item,
    x: 44 + (index / Math.max(props.data.length - 1, 1)) * width,
    y: 228 - ((item.value - min) / range) * 196,
  }))
})

const linePath = computed(() => points.value
  .map((point, index) => `${index ? 'L' : 'M'} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
  .join(' '))

const areaPath = computed(() => {
  if (!points.value.length) return ''
  const first = points.value[0]!
  const last = points.value[points.value.length - 1]!
  return `${linePath.value} L ${last.x.toFixed(2)} 228 L ${first.x.toFixed(2)} 228 Z`
})

const ariaLabel = computed(() => {
  if (!props.data.length) return `${props.title || '趋势图'}，暂无数据`
  const total = props.data.reduce((sum, item) => sum + item.value, 0)
  return `${props.title || '趋势图'}，共 ${props.data.length} 个数据点，合计 ${total}`
})
</script>

<style scoped>
.line-chart {
  position: relative;
  min-height: 280px;
}

svg {
  width: 100%;
  height: 250px;
  overflow: visible;
}

.grid line {
  stroke: var(--app-border-subtle, #e8eaed);
  stroke-width: 1;
}

.line {
  fill: none;
  stroke: var(--app-color-primary, #2563eb);
  stroke-width: 3;
  vector-effect: non-scaling-stroke;
}

.area {
  fill: color-mix(in srgb, var(--app-color-primary, #2563eb) 14%, transparent);
}

circle {
  fill: var(--app-surface, #fff);
  stroke: var(--app-color-primary, #2563eb);
  stroke-width: 3;
  vector-effect: non-scaling-stroke;
}

.axis-labels {
  display: flex;
  justify-content: space-between;
  padding: 0 12px 0 44px;
  color: var(--app-text-muted, #6b7280);
  font-size: 12px;
}

.empty {
  display: grid;
  min-height: 260px;
  place-items: center;
  color: var(--app-text-muted, #6b7280);
}

@media (prefers-reduced-motion: reduce) {
  .line,
  .area,
  circle {
    transition: none;
  }
}
</style>

