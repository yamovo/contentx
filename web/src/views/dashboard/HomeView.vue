<template>
  <div class="home-page">
    <header class="hero">
      <nav class="nav">
        <h1 class="logo">VortexCMS</h1>
        <div class="nav-links">
          <router-link to="/login">登录</router-link>
        </div>
      </nav>
      <div class="hero-content">
        <h1>现代化内容管理系统</h1>
        <p>基于 Go + Vue 3 构建的高性能、可扩展的 CMS</p>
        <div class="hero-actions">
          <el-button type="primary" size="large" @click="$router.push('/login')">进入后台</el-button>
          <el-button size="large" @click="scrollToFeatures">了解更多</el-button>
        </div>
      </div>
    </header>

    <section class="features">
      <div class="container">
        <h2>核心特性</h2>
        <el-row :gutter="24">
          <el-col :span="8" v-for="f in features" :key="f.title">
            <div class="feature-card">
              <el-icon :size="40" :color="f.color"><component :is="f.icon" /></el-icon>
              <h3>{{ f.title }}</h3>
              <p>{{ f.desc }}</p>
            </div>
          </el-col>
        </el-row>
      </div>
    </section>

    <footer class="footer">
      <p>© 2026 ContentX. Powered by Go + Vue 3</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { animate } from 'animejs'
import { stagger } from 'animejs/utils'
const heroRef = ref<HTMLElement>()
const featuresRef = ref<HTMLElement>()

const features = [
  { title: '高性能', desc: 'Go 后端，编译型语言带来的极致性能', icon: 'Lightning', color: '#e6a23c' },
  { title: 'Markdown 支持', desc: '原生 Markdown 编辑器，所见即所得', icon: 'EditPen', color: '#409eff' },
  { title: 'SEO 优化', desc: '内置 SEO 工具，自动生成 Sitemap', icon: 'Search', color: '#67c23a' },
  { title: 'RBAC 权限', desc: '细粒度角色权限控制', icon: 'Lock', color: '#909399' },
  { title: '插件系统', desc: '可扩展的插件架构', icon: 'Connection', color: '#f56c6c' },
  { title: '数据分析', desc: '内置访问统计和分析报表', icon: 'TrendCharts', color: '#764ba2' },
]

onMounted(() => {
  // Hero text entrance
  animate('.hero-content h1', {
    opacity: { from: 0 },
    translateY: { from: 40 },
    duration: 900,
    ease: 'outQuint',
  })
  animate('.hero-content p', {
    opacity: { from: 0 },
    translateY: { from: 24 },
    duration: 800,
    delay: 200,
    ease: 'outQuint',
  })
  animate('.hero-actions', {
    opacity: { from: 0 },
    translateY: { from: 16 },
    duration: 700,
    delay: 400,
    ease: 'outQuint',
  })

  // Floating background circles
  animate('.hero::before', {
    scale: [1, 1.1],
    duration: 4000,
    loop: true,
    alternate: true,
    ease: 'inOutSine',
  })

  // Feature cards stagger (triggered by scroll observer or just on mount)
  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        animate('.feature-card', {
          opacity: { from: 0 },
          translateY: { from: 30 },
          scale: { from: 0.95 },
          duration: 600,
          delay: stagger(100),
          ease: 'outQuint',
        })
        observer.disconnect()
      }
    })
  }, { threshold: 0.2 })
  const featEl = document.querySelector('.features')
  if (featEl) observer.observe(featEl)
})

function scrollToFeatures() {
  document.querySelector('.features')?.scrollIntoView({ behavior: 'smooth' })
}
</script>

<style lang="scss" scoped>
.home-page {
  .hero {
    min-height: 100vh;
    background: linear-gradient(135deg, #1d1e2c 0%, #2d3561 100%);
    color: #fff;
    .nav {
      display: flex; justify-content: space-between; align-items: center;
      padding: 20px 60px;
      .logo { font-size: 24px; font-weight: 700; margin: 0; }
      a { color: rgba(255,255,255,0.8); text-decoration: none; &:hover { color: #fff; } }
    }
    .hero-content {
      text-align: center; padding: 120px 20px 0;
      h1 { font-size: 48px; margin-bottom: 16px; }
      p { font-size: 20px; color: rgba(255,255,255,0.7); margin-bottom: 32px; }
      .hero-actions { display: flex; gap: 16px; justify-content: center; }
    }
  }
  .features {
    padding: 80px 0;
    .container { max-width: 1000px; margin: 0 auto; padding: 0 20px; }
    h2 { text-align: center; font-size: 32px; margin-bottom: 48px; }
    .feature-card {
      text-align: center; padding: 32px 20px;
      h3 { margin: 16px 0 8px; font-size: 18px; }
      p { color: #606266; font-size: 14px; }
    }
  }
  .footer {
    text-align: center; padding: 24px; background: #1d1e2c; color: rgba(255,255,255,0.5); font-size: 14px;
  }
}
</style>
