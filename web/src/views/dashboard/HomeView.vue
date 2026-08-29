<template>
  <div class="home-page">
    <header class="hero">
      <nav class="nav">
        <h1 class="logo">
          ContentX
        </h1>
        <div class="nav-links">
          <router-link to="/login">
            登录
          </router-link>
        </div>
      </nav>
      <div class="hero-content">
        <h1>面向开发者和 AI Agent 的自托管 Headless CMS</h1>
        <p>用 REST、GraphQL 与 MCP 连接网站、应用和 Agent，内容与数据始终由你掌控。</p>
        <div class="hero-actions">
          <el-button
            type="primary"
            size="large"
            @click="$router.push('/login')"
          >
            进入后台
          </el-button>
          <el-button
            size="large"
            @click="scrollToFeatures"
          >
            了解更多
          </el-button>
        </div>
      </div>
    </header>

    <section class="features">
      <div class="container">
        <h2>为什么选择 ContentX</h2>
        <el-row :gutter="24">
          <el-col
            v-for="f in features"
            :key="f.title"
            :span="8"
          >
            <div class="feature-card">
              <el-icon
                :size="40"
                :color="f.color"
              >
                <component :is="f.icon" />
              </el-icon>
              <h3>{{ f.title }}</h3>
              <p>{{ f.desc }}</p>
            </div>
          </el-col>
        </el-row>
      </div>
    </section>

    <section class="comparison">
      <div class="container">
        <h2>差异不只是功能数量</h2>
        <p class="section-lead">
          ContentX 选择更适合开发团队与 AI 工作流的技术路线。
        </p>
        <el-row :gutter="24">
          <el-col
            v-for="item in differentiators"
            :key="item.title"
            :span="8"
          >
            <div class="comparison-card">
              <span>{{ item.context }}</span>
              <h3>{{ item.title }}</h3>
              <p>{{ item.desc }}</p>
            </div>
          </el-col>
        </el-row>
      </div>
    </section>

    <footer class="footer">
      <p>© 2026 ContentX · 自托管 · Agent-ready</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useAnime } from '@/composables/useAnime'

// Tracked animations are cancelled on unmount.
const { animate, stagger } = useAnime()
let featuresObserver: IntersectionObserver | null = null

const features = [
  { title: '语言中立契约', desc: '核心当前使用 Go，消费者通过 REST、OpenAPI 与 MCP 接入，不依赖内部语言类型', icon: 'Lightning', color: '#e6a23c' },
  { title: '数据主权', desc: '部署在自己的基础设施中，数据、权限与升级节奏由团队掌控', icon: 'DataAnalysis', color: '#409eff' },
  { title: 'Agent-ready', desc: '通过 MCP 为 AI Agent 提供受权限控制的内容读写与发布能力', icon: 'Connection', color: '#67c23a' },
  { title: '多协议 API', desc: 'REST、只读 GraphQL 与 OpenAPI，让网站、应用和工具按需接入', icon: 'Link', color: '#909399' },
  { title: '安全权限边界', desc: 'RBAC、API Token、审计日志与发布工作流共同保护内容操作', icon: 'Lock', color: '#f56c6c' },
  { title: '完整内容工作台', desc: '内容建模、媒体、SEO、Webhook 与分析能力集中在一个后台', icon: 'Grid', color: '#764ba2' },
]

const differentiators = [
  {
    context: '对比主流 Node.js CMS',
    title: 'Go 技术栈更利于自托管',
    desc: '减少运行时依赖，以编译型后端获得更直接的部署和运维体验。',
  },
  {
    context: '对比云端 SaaS CMS',
    title: '基础设施与数据留在自己手中',
    desc: '不被托管平台绑定，部署位置、数据边界和升级窗口都由团队决定。',
  },
  {
    context: '对比只服务人工后台的 CMS',
    title: '开发者与 Agent 共用内容底座',
    desc: 'REST、GraphQL 与 MCP 并行，让人、应用和 AI Agent 在权限边界内协作。',
  },
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

  // Feature cards stagger (triggered by scroll observer or just on mount)
  featuresObserver = new IntersectionObserver((entries) => {
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
        featuresObserver?.disconnect()
        featuresObserver = null
      }
    })
  }, { threshold: 0.2 })
  const featEl = document.querySelector('.features')
  if (featEl) featuresObserver.observe(featEl)
})

onUnmounted(() => {
  featuresObserver?.disconnect()
  featuresObserver = null
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
  .comparison {
    padding: 80px 0;
    background: #f5f7fa;
    .container { max-width: 1000px; margin: 0 auto; padding: 0 20px; }
    h2 { text-align: center; font-size: 32px; margin-bottom: 12px; }
    .section-lead { text-align: center; color: #606266; margin: 0 0 40px; }
    .comparison-card {
      height: 100%;
      box-sizing: border-box;
      padding: 28px;
      border: 1px solid #e4e7ed;
      border-radius: 12px;
      background: #fff;
      span { color: #409eff; font-size: 13px; font-weight: 600; }
      h3 { margin: 12px 0 10px; font-size: 18px; }
      p { margin: 0; color: #606266; font-size: 14px; line-height: 1.7; }
    }
  }
  .footer {
    text-align: center; padding: 24px; background: #1d1e2c; color: rgba(255,255,255,0.5); font-size: 14px;
  }
}

@media (max-width: 768px) {
  .home-page {
    .hero {
      .nav { padding: 20px 24px; }
      .hero-content {
        padding-top: 88px;
        h1 { font-size: 36px; }
      }
    }
    .features,
    .comparison {
      :deep(.el-col) { max-width: 100%; flex: 0 0 100%; margin-bottom: 20px; }
    }
  }
}
</style>
