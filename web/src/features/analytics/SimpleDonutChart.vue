<template>
  <div
    v-if="segments.length"
    class="donut-layout"
    role="img"
    :aria-label="ariaLabel"
  >
    <div
      class="donut"
      :style="{ background: gradient }"
      aria-hidden="true"
    >
      <div class="donut-hole">
        <strong>{{ total }}</strong>
        <span>总计</span>
      </div>
    </div>
    <ul class="legend">
      <li
        v-for="segment in segments"
        :key="segment.name"
      >
        <span
          class="swatch"
          :style="{ background: segment.color }"
        />
        <span>{{ segment.name }}</span>
        <strong>{{ segment.percent }}%</strong>
      </li>
    </ul>
  </div>
  <div
    v-else
    class="empty"
    role="status"
  >
    暂无设备数据
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Datum {
  name: string
  value: number
}

const props = defineProps<{
  data: Datum[]
  title?: string
}>()

const palette = ['#2563eb', '#14b8a6', '#f59e0b', '#8b5cf6', '#ef4444', '#64748b']
const total = computed(() => props.data.reduce((sum, item) => sum + Math.max(0, item.value), 0))
const segments = computed(() => props.data
  .filter(item => item.value > 0)
  .map((item, index) => ({
    ...item,
    color: palette[index % palette.length]!,
    percent: total.value ? Math.round((item.value / total.value) * 100) : 0,
  })))

const gradient = computed(() => {
  let cursor = 0
  const stops = segments.value.map(segment => {
    const start = cursor
    cursor += segment.percent
    return `${segment.color} ${start}% ${cursor}%`
  })
  return `conic-gradient(${stops.join(', ')})`
})

const ariaLabel = computed(() => `${props.title || '分布图'}：${
  segments.value.map(segment => `${segment.name} ${segment.percent}%`).join('，')
}`)
</script>

<style scoped>
.donut-layout {
  display: grid;
  min-height: 280px;
  grid-template-columns: minmax(150px, 210px) 1fr;
  align-items: center;
  gap: 28px;
}

.donut {
  aspect-ratio: 1;
  width: min(100%, 210px);
  border-radius: 50%;
  display: grid;
  place-items: center;
}

.donut-hole {
  width: 58%;
  aspect-ratio: 1;
  border-radius: 50%;
  background: var(--app-surface, #fff);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.donut-hole strong {
  color: var(--app-text-strong, #111827);
  font-size: 24px;
}

.donut-hole span {
  color: var(--app-text-muted, #6b7280);
  font-size: 12px;
}

.legend {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.legend li {
  display: grid;
  grid-template-columns: 10px 1fr auto;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.swatch {
  width: 10px;
  height: 10px;
  border-radius: 3px;
}

.empty {
  display: grid;
  min-height: 280px;
  place-items: center;
  color: var(--app-text-muted, #6b7280);
}

@media (max-width: 1160px) {
  .donut-layout {
    grid-template-columns: 1fr;
    justify-items: center;
  }

  .legend {
    width: 100%;
  }
}
</style>
