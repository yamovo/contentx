<template>
  <div class="not-found-page" ref="pageRef">
    <div class="nf-content" ref="contentRef">
      <div class="nf-code" ref="codeRef">404</div>
      <h1 ref="titleRef">Page Not Found</h1>
      <p ref="descRef">The page you are looking for does not exist or has been moved.</p>
      <div class="nf-actions" ref="actionsRef">
        <router-link to="/" class="nf-btn primary">Go Home</router-link>
        <router-link to="/blog" class="nf-btn">Browse Articles</router-link>
      </div>
    </div>
    <div class="nf-bg">
      <div class="nf-circle c1"></div>
      <div class="nf-circle c2"></div>
      <div class="nf-circle c3"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { animate, createTimeline } from 'animejs'

const pageRef = ref<HTMLElement>()
const contentRef = ref<HTMLElement>()
const codeRef = ref<HTMLElement>()
const titleRef = ref<HTMLElement>()
const descRef = ref<HTMLElement>()
const actionsRef = ref<HTMLElement>()

onMounted(() => {
  // Animated 404 entrance
  if (codeRef.value) {
    animate(codeRef.value, {
      scale: { from: 0.5 },
      opacity: { from: 0 },
      rotate: { from: -10 },
      duration: 1000,
      ease: 'outElastic(1, 0.5)',
    })
  }

  // Stagger text elements
  const tl = createTimeline({ defaults: { ease: 'outQuint' } })
  if (titleRef.value) {
    tl.add(titleRef.value, { opacity: { from: 0 }, translateY: { from: 20 }, duration: 500 }, 300)
  }
  if (descRef.value) {
    tl.add(descRef.value, { opacity: { from: 0 }, translateY: { from: 16 }, duration: 500 }, 500)
  }
  if (actionsRef.value) {
    tl.add(actionsRef.value, { opacity: { from: 0 }, translateY: { from: 16 }, duration: 500 }, 700)
  }

  // Floating circles
  document.querySelectorAll('.nf-circle').forEach((el, i) => {
    animate(el as HTMLElement, {
      translateY: [{ to: -15 - i * 5, duration: 2000 + i * 300 }, { to: 15 + i * 5, duration: 2000 + i * 300 }],
      translateX: [{ to: 10 + i * 3, duration: 2500 }, { to: -10 - i * 3, duration: 2500 }],
      ease: 'inOutSine',
      loop: true,
      alternate: true,
    })
  })
})
</script>

<style lang="scss" scoped>
.not-found-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-primary, #f5f7fa);
  position: relative;
  overflow: hidden;
}

.nf-content {
  text-align: center;
  z-index: 1;
  padding: 40px;
}

.nf-code {
  font-size: 160px;
  font-weight: 900;
  line-height: 1;
  background: linear-gradient(135deg, #409eff, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 16px;
  display: inline-block;
}

.nf-content h1 {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary, #303133);
  margin-bottom: 12px;
}

.nf-content p {
  font-size: 16px;
  color: var(--text-muted, #909399);
  margin-bottom: 32px;
  max-width: 400px;
  margin-left: auto;
  margin-right: auto;
}

.nf-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}

.nf-btn {
  display: inline-block;
  padding: 12px 28px;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.2s;
  border: 1px solid var(--border-color, #ebeef5);
  color: var(--text-primary, #303133);
  background: var(--bg-card, #fff);
  &:hover { border-color: #409eff; color: #409eff; }
  &.primary {
    background: #409eff;
    color: #fff;
    border-color: #409eff;
    &:hover { background: #337ecc; }
  }
}

.nf-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.nf-circle {
  position: absolute;
  border-radius: 50%;
  background: rgba(64, 158, 255, 0.06);
  &.c1 { width: 300px; height: 300px; top: 10%; left: 10%; }
  &.c2 { width: 200px; height: 200px; bottom: 15%; right: 15%; background: rgba(118, 75, 162, 0.06); }
  &.c3 { width: 150px; height: 150px; top: 50%; right: 30%; }
}

@media (max-width: 600px) {
  .nf-code { font-size: 100px; }
  .nf-content h1 { font-size: 22px; }
}
</style>
