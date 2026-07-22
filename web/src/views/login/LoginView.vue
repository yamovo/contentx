<template>
  <div class="login-page">
    <div class="login-container">
      <div class="login-card" ref="cardRef">
        <div class="login-header">
          <h1 class="logo-text" ref="logoRef">ContentX</h1>
          <p class="subtitle">内容管理系统</p>
        </div>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-width="0"
          @submit.prevent="handleLogin"
        >
          <el-form-item prop="username">
            <el-input
              v-model="form.username"
              placeholder="用户名或邮箱"
              :prefix-icon="User"
              size="large"
              clearable
            />
          </el-form-item>

          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="密码"
              :prefix-icon="Lock"
              size="large"
              show-password
              @keyup.enter="handleLogin"
            />
          </el-form-item>

          <el-form-item>
            <div class="login-options">
              <el-checkbox v-model="rememberMe">记住我</el-checkbox>
              <a href="#" class="forgot-link">忘记密码？</a>
            </div>
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="authStore.loading"
              @click="handleLogin"
              class="login-btn"
            >
              登 录
            </el-button>
          </el-form-item>
        </el-form>

        <div class="login-footer">
          <span>还没有账号？</span>
          <router-link to="/register">立即注册</router-link>
        </div>
      </div>
    </div>

    <div class="login-bg">
      <div class="bg-overlay" ref="bgRef"></div>
      <!-- floating decorative circles -->
      <div class="bg-circle c1" ref="c1Ref"></div>
      <div class="bg-circle c2" ref="c2Ref"></div>
      <div class="bg-circle c3" ref="c3Ref"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { User, Lock } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { animate, createTimeline } from 'animejs'
import { stagger } from 'animejs/utils'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref<FormInstance>()
const cardRef = ref<HTMLElement>()
const logoRef = ref<HTMLElement>()
const bgRef = ref<HTMLElement>()
const c1Ref = ref<HTMLElement>()
const c2Ref = ref<HTMLElement>()
const c3Ref = ref<HTMLElement>()

const form = reactive({
  username: '',
  password: '',
})
const rememberMe = ref(false)

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6个字符', trigger: 'blur' },
  ],
}

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  try {
    await authStore.login(form.username, form.password)
    ElMessage.success('登录成功')

    const redirect = (route.query.redirect as string) || '/admin'
    router.push(redirect)
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '登录失败')
  }
}

onMounted(() => {
  // Card entrance: slide up + fade in
  if (cardRef.value) {
    animate(cardRef.value, {
      opacity: { from: 0 },
      translateY: { from: 40 },
      duration: 800,
      ease: 'outQuint',
    })
  }

  // Logo text bounce
  if (logoRef.value) {
    animate(logoRef.value, {
      scale: { from: 0.7 },
      opacity: { from: 0 },
      duration: 1000,
      delay: 200,
      ease: 'outElastic(1, 0.6)',
    })
  }

  // Background gradient pulse
  if (bgRef.value) {
    animate(bgRef.value, {
      opacity: [0, 0.15],
      duration: 1500,
      delay: 300,
      ease: 'inOutQuad',
      alternate: true,
      loop: true,
    })
  }

  // Floating circles – continuous gentle drift
  const circles = [c1Ref.value, c2Ref.value, c3Ref.value].filter(Boolean)
  circles.forEach((el, i) => {
    if (!el) return
    animate(el, {
      translateY: [
        { to: -20 - i * 8, duration: 2000 + i * 400 },
        { to: 20 + i * 8, duration: 2000 + i * 400 },
      ],
      translateX: [
        { to: 15 + i * 5, duration: 2500 + i * 300 },
        { to: -15 - i * 5, duration: 2500 + i * 300 },
      ],
      scale: [
        { to: 1.05 + i * 0.02, duration: 3000 },
        { to: 0.95 - i * 0.02, duration: 3000 },
      ],
      ease: 'inOutSine',
      loop: true,
      alternate: true,
    })
  })
})
</script>

<style lang="scss" scoped>
.login-page {
  min-height: 100vh;
  display: flex;
}

.login-container {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  background: #fff;
  z-index: 1;
}

.login-card {
  width: 100%;
  max-width: 400px;

  .login-header {
    text-align: center;
    margin-bottom: 40px;

    .logo-text {
      font-size: 32px;
      font-weight: 700;
      color: #1d1e2c;
      margin-bottom: 8px;
      display: inline-block; /* needed for scale transform */
    }

    .subtitle {
      color: #909399;
      font-size: 14px;
    }
  }

  .login-options {
    display: flex;
    justify-content: space-between;
    width: 100%;

    .forgot-link {
      color: #409eff;
      text-decoration: none;
      font-size: 14px;
    }
  }

  .login-btn {
    width: 100%;
  }

  .login-footer {
    text-align: center;
    margin-top: 20px;
    font-size: 14px;
    color: #909399;

    a {
      color: #409eff;
      text-decoration: none;
      margin-left: 4px;
    }
  }
}

.login-bg {
  flex: 1;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  overflow: hidden;

  .bg-overlay {
    position: absolute;
    inset: 0;
    background: url('@/assets/login-pattern.svg') center/cover no-repeat;
    opacity: 0;
  }

  .bg-circle {
    position: absolute;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.08);
    backdrop-filter: blur(2px);

    &.c1 {
      width: 300px; height: 300px;
      top: 10%; left: 20%;
    }
    &.c2 {
      width: 200px; height: 200px;
      top: 55%; left: 60%;
    }
    &.c3 {
      width: 150px; height: 150px;
      top: 30%; left: 70%;
    }
  }
}

@media (max-width: 768px) {
  .login-bg { display: none; }
  .login-container { padding: 20px; }
}
</style>
