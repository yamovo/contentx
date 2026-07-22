<template>
  <div class="blog-layout" :class="{ 'dark-mode': isDark }">
    <!-- Navbar -->
    <header class="blog-nav" ref="navRef">
      <div class="nav-inner">
        <router-link to="/" class="nav-logo">
          <span class="logo-text">VortexCMS</span>
        </router-link>
        <nav class="nav-links" :class="{ open: mobileMenuOpen }">
          <router-link to="/blog" @click="closeMobile">文章</router-link>
          <router-link to="/blog/categories" @click="closeMobile">分类</router-link>
          <router-link to="/blog/tags" @click="closeMobile">标签</router-link>
        </nav>
        <div class="nav-right">
          <button class="theme-btn" @click="toggleTheme" :title="isDark ? 'Light mode' : 'Dark mode'">
            <span v-if="isDark">&#9788;</span>
            <span v-else>&#9789;</span>
          </button>
          <button class="mobile-toggle" @click="mobileMenuOpen = !mobileMenuOpen">
            <span></span><span></span><span></span>
          </button>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="blog-main">
      <router-view />
    </main>

    <!-- Footer -->
    <footer class="blog-footer">
      <div class="footer-inner">
        <div class="footer-brand">
          <strong>ContentX</strong>
          <p>Modern content management powered by Go + Vue 3</p>
        </div>
        <div class="footer-links">
          <div>
            <h4>Navigation</h4>
            <router-link to="/blog">Articles</router-link>
            <router-link to="/blog/categories">Categories</router-link>
            <router-link to="/blog/tags">Tags</router-link>
          </div>
          <div>
            <h4>Admin</h4>
            <router-link to="/login">Login</router-link>
            <router-link to="/admin">Dashboard</router-link>
          </div>
        </div>
        <div class="footer-bottom">
          <p>&copy; {{ new Date().getFullYear() }} ContentX. All rights reserved.</p>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { animate } from 'animejs'

const mobileMenuOpen = ref(false)
const navRef = ref<HTMLElement>()
const isDark = ref(false)

function closeMobile() {
  mobileMenuOpen.value = false
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light')
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

onMounted(() => {
  const saved = localStorage.getItem('theme')
  if (saved === 'dark') {
    isDark.value = true
    document.documentElement.classList.add('dark')
    document.documentElement.setAttribute('data-theme', 'dark')
  }

  // Animate navbar entrance
  if (navRef.value) {
    animate(navRef.value, {
      opacity: { from: 0 },
      translateY: { from: -20 },
      duration: 600,
      ease: 'outQuint',
    })
  }
})
</script>

<style lang="scss" scoped>
.blog-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-primary, #f5f7fa);
  color: var(--text-primary, #303133);
}

// Navbar
.blog-nav {
  position: sticky;
  top: 0;
  z-index: 100;
  background: rgba(255,255,255,0.85);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--border-color, #ebeef5);

  .dark-mode & {
    background: rgba(26,26,46,0.9);
  }

  .nav-inner {
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 24px;
    height: 60px;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .nav-logo {
    text-decoration: none;
    .logo-text {
      font-size: 20px;
      font-weight: 700;
      background: linear-gradient(135deg, #409eff, #764ba2);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }
  }

  .nav-links {
    display: flex;
    gap: 32px;
    a {
      color: var(--text-secondary, #606266);
      text-decoration: none;
      font-size: 15px;
      font-weight: 500;
      transition: color 0.2s;
      &:hover, &.router-link-active {
        color: #409eff;
      }
    }
  }

  .nav-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .theme-btn {
    background: none;
    border: 1px solid var(--border-color, #ebeef5);
    border-radius: 8px;
    width: 36px;
    height: 36px;
    cursor: pointer;
    font-size: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary, #606266);
    transition: all 0.2s;
    &:hover { border-color: #409eff; color: #409eff; }
  }

  .mobile-toggle {
    display: none;
    flex-direction: column;
    gap: 5px;
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    span {
      width: 22px;
      height: 2px;
      background: var(--text-primary, #303133);
      border-radius: 1px;
      transition: 0.2s;
    }
  }
}

// Main
.blog-main {
  flex: 1;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 32px 24px;
}

// Footer
.blog-footer {
  background: var(--sidebar-bg, #1d1e2c);
  color: rgba(255,255,255,0.7);
  padding: 48px 24px 24px;
  margin-top: auto;

  .footer-inner {
    max-width: 1200px;
    margin: 0 auto;
  }

  .footer-brand {
    margin-bottom: 32px;
    strong { font-size: 20px; color: #fff; }
    p { margin-top: 8px; font-size: 14px; color: rgba(255,255,255,0.5); }
  }

  .footer-links {
    display: flex;
    gap: 64px;
    margin-bottom: 32px;
    h4 { color: #fff; margin-bottom: 12px; font-size: 14px; text-transform: uppercase; letter-spacing: 1px; }
    a {
      display: block;
      color: rgba(255,255,255,0.5);
      text-decoration: none;
      font-size: 14px;
      margin-bottom: 8px;
      &:hover { color: #409eff; }
    }
  }

  .footer-bottom {
    border-top: 1px solid rgba(255,255,255,0.08);
    padding-top: 20px;
    p { font-size: 13px; color: rgba(255,255,255,0.3); }
  }
}

@media (max-width: 768px) {
  .blog-nav {
    .nav-links {
      display: none;
      position: absolute;
      top: 60px;
      left: 0;
      right: 0;
      background: var(--bg-secondary, #fff);
      flex-direction: column;
      padding: 16px 24px;
      gap: 16px;
      border-bottom: 1px solid var(--border-color, #ebeef5);
      box-shadow: 0 8px 24px rgba(0,0,0,0.08);
      .dark-mode & { background: var(--bg-secondary, #16213e); }
      &.open { display: flex; }
    }
    .mobile-toggle { display: flex; }
  }
  .blog-footer .footer-links { flex-direction: column; gap: 24px; }
}
</style>
