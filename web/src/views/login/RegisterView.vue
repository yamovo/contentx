<template>
  <div class="login-page">
    <div class="login-container">
      <div class="login-card">
        <div class="login-header">
          <h1 class="logo-text">ContentX</h1>
          <p class="subtitle">创建新账号</p>
        </div>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-width="0"
          @submit.prevent="handleRegister"
        >
          <el-form-item prop="username">
            <el-input
              v-model="form.username"
              placeholder="用户名"
              :prefix-icon="User"
              size="large"
              clearable
            />
          </el-form-item>

          <el-form-item prop="email">
            <el-input
              v-model="form.email"
              placeholder="邮箱"
              :prefix-icon="Message"
              size="large"
              clearable
            />
          </el-form-item>

          <el-form-item prop="display_name">
            <el-input
              v-model="form.display_name"
              placeholder="显示名称（可选）"
              :prefix-icon="UserFilled"
              size="large"
              clearable
            />
          </el-form-item>

          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="密码（至少6位）"
              :prefix-icon="Lock"
              size="large"
              show-password
            />
          </el-form-item>

          <el-form-item prop="confirmPassword">
            <el-input
              v-model="form.confirmPassword"
              type="password"
              placeholder="确认密码"
              :prefix-icon="Lock"
              size="large"
              show-password
              @keyup.enter="handleRegister"
            />
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="loading"
              @click="handleRegister"
              class="login-btn"
            >
              注 册
            </el-button>
          </el-form-item>
        </el-form>

        <div class="login-footer">
          <span>已有账号？</span>
          <router-link to="/login">立即登录</router-link>
        </div>
      </div>
    </div>

    <div class="login-bg">
      <div class="bg-overlay"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { User, Lock, Message, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  username: '',
  email: '',
  display_name: '',
  password: '',
  confirmPassword: '',
})

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 64, message: '用户名长度3-64个字符', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6个字符', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    {
      validator: (_r: any, value: string, callback: Function) => {
        if (value !== form.password) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
}

async function handleRegister() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    await authStore.register({
      username: form.username,
      email: form.email,
      password: form.password,
      display_name: form.display_name || undefined,
    })
    ElMessage.success('注册成功')
    router.push('/admin')
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '注册失败')
  } finally {
    loading.value = false
  }
}
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
    }

    .subtitle {
      color: #909399;
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

  .bg-overlay {
    position: absolute;
    inset: 0;
    background: url('@/assets/login-pattern.svg') center/cover no-repeat;
    opacity: 0.1;
  }
}

@media (max-width: 768px) {
  .login-bg { display: none; }
  .login-container { padding: 20px; }
}
</style>
