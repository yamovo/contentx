<template>
  <div class="profile-page">
    <h2>个人资料</h2>

    <el-row :gutter="20">
      <!-- Left: avatar + summary -->
      <el-col :span="8">
        <el-card shadow="never">
          <div class="profile-summary">
            <el-avatar
              :src="form.avatar"
              :size="96"
            >
              {{ (form.display_name || 'U')[0] }}
            </el-avatar>
            <h3>{{ form.display_name || form.username }}</h3>
            <p class="meta">
              @{{ form.username }}
            </p>
            <el-tag
              v-if="authStore.user?.role"
              size="small"
            >
              {{ authStore.user.role.name }}
            </el-tag>
            <p
              v-if="authStore.user?.created_at"
              class="meta"
            >
              注册于 {{ formatDate(authStore.user.created_at, 'YYYY-MM-DD') }}
            </p>
          </div>
        </el-card>
      </el-col>

      <!-- Right: editable form -->
      <el-col :span="16">
        <el-card shadow="never">
          <template #header>
            <span>基本信息</span>
          </template>
          <el-form
            ref="formRef"
            :model="form"
            :rules="rules"
            label-width="100px"
            label-position="left"
          >
            <el-form-item
              label="用户名"
              prop="username"
            >
              <el-input
                v-model="form.username"
                disabled
              />
            </el-form-item>
            <el-form-item
              label="昵称"
              prop="display_name"
            >
              <el-input v-model="form.display_name" />
            </el-form-item>
            <el-form-item
              label="邮箱"
              prop="email"
            >
              <el-input
                v-model="form.email"
                disabled
              />
            </el-form-item>
            <el-form-item
              label="头像 URL"
              prop="avatar"
            >
              <el-input
                v-model="form.avatar"
                placeholder="https://..."
              />
            </el-form-item>
            <el-form-item
              label="个人网站"
              prop="website"
            >
              <el-input
                v-model="form.website"
                placeholder="https://..."
              />
            </el-form-item>
            <el-form-item
              label="个人简介"
              prop="bio"
            >
              <el-input
                v-model="form.bio"
                type="textarea"
                :rows="3"
                maxlength="200"
                show-word-limit
              />
            </el-form-item>
            <el-form-item>
              <el-button
                type="primary"
                :loading="updateProfile.isPending.value"
                @click="onSave"
              >
                保存修改
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card
          shadow="never"
          class="password-card"
        >
          <template #header>
            <span>修改密码</span>
          </template>
          <el-form
            ref="pwdFormRef"
            :model="pwd"
            :rules="pwdRules"
            label-width="100px"
            label-position="left"
          >
            <el-form-item
              label="当前密码"
              prop="old_password"
            >
              <el-input
                v-model="pwd.old_password"
                type="password"
                show-password
              />
            </el-form-item>
            <el-form-item
              label="新密码"
              prop="new_password"
            >
              <el-input
                v-model="pwd.new_password"
                type="password"
                show-password
              />
            </el-form-item>
            <el-form-item
              label="确认新密码"
              prop="confirm_password"
            >
              <el-input
                v-model="pwd.confirm_password"
                type="password"
                show-password
              />
            </el-form-item>
            <el-form-item>
              <el-button
                type="primary"
                :loading="changePassword.isPending.value"
                @click="onChangePassword"
              >
                修改密码
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- Two-factor authentication -->
        <el-card
          shadow="never"
          class="password-card"
        >
          <template #header>
            <div class="totp-header">
              <span>两步验证 (TOTP)</span>
              <el-tag
                :type="totpEnabled ? 'success' : 'info'"
                size="small"
              >
                {{ totpEnabled ? '已启用' : '未启用' }}
              </el-tag>
            </div>
          </template>

          <!-- Not enabled: entry point -->
          <template v-if="!totpEnabled && !totpSetup">
            <p class="totp-desc">
              启用后，登录除密码外还需输入身份验证器 App（如 Google Authenticator）生成的动态验证码。
            </p>
            <el-button
              type="primary"
              :loading="totpLoading"
              @click="startTotpSetup"
            >
              开始绑定
            </el-button>
          </template>

          <!-- Setup phase: show secret, wait for the confirmation code -->
          <template v-else-if="totpSetup">
            <el-alert
              type="warning"
              :closable="false"
              show-icon
            >
              在身份验证器 App 中选择“手动输入密钥”，填入下方密钥（或粘贴 otpauth 链接），然后输入 App 显示的 6 位验证码完成绑定。
            </el-alert>
            <div class="totp-secret">
              <div>
                密钥：<code>{{ totpSetup.secret }}</code>
                <el-button
                  text
                  size="small"
                  @click="copyText(totpSetup.secret)"
                >
                  复制
                </el-button>
              </div>
              <div class="totp-uri">
                otpauth 链接：<code>{{ totpSetup.otpauth_uri }}</code>
                <el-button
                  text
                  size="small"
                  @click="copyText(totpSetup.otpauth_uri)"
                >
                  复制
                </el-button>
              </div>
            </div>
            <div class="totp-confirm">
              <el-input
                v-model="totpCode"
                placeholder="6 位验证码"
                maxlength="6"
                style="width: 160px"
              />
              <el-button
                type="primary"
                :loading="totpLoading"
                @click="confirmTotpEnable"
              >
                确认启用
              </el-button>
              <el-button @click="totpSetup = null">
                取消
              </el-button>
            </div>
          </template>

          <!-- Enabled: disable entry -->
          <template v-else>
            <p class="totp-desc">
              两步验证已启用。如需更换设备，请先解除绑定后重新绑定。
            </p>
            <div class="totp-confirm">
              <el-input
                v-model="totpPassword"
                type="password"
                placeholder="输入账号密码确认"
                show-password
                style="width: 220px"
              />
              <el-button
                type="danger"
                :loading="totpLoading"
                @click="disableTotp"
              >
                解除绑定
              </el-button>
            </div>
          </template>
        </el-card>
      </el-col>
    </el-row>

    <!-- One-time backup codes -->
    <el-dialog
      v-model="backupVisible"
      title="备用恢复码"
      width="480px"
      :close-on-click-modal="false"
    >
      <el-alert
        type="warning"
        :closable="false"
        show-icon
      >
        每个备用码只能使用一次，丢失身份验证器时可用它登录。请立即保存，关闭后不再显示。
      </el-alert>
      <div class="backup-codes">
        <code
          v-for="c in backupCodes"
          :key="c"
        >{{ c }}</code>
      </div>
      <template #footer>
        <el-button @click="copyText(backupCodes.join('\n'))">
          复制全部
        </el-button>
        <el-button
          type="primary"
          @click="backupVisible = false"
        >
          我已保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'
import { totpApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { formatDate, getApiError } from '@/utils'
import { useProfileMutation } from '@/features/settings/use-profile-mutation'
import { validatePasswordStrength } from '@/shared/auth/password'

const authStore = useAuthStore()
const { updateProfile, changePassword } = useProfileMutation()

const formRef = ref<FormInstance>()
const pwdFormRef = ref<FormInstance>()

const form = reactive({
  username: authStore.user?.username || '',
  display_name: authStore.user?.display_name || '',
  email: authStore.user?.email || '',
  avatar: authStore.user?.avatar || '',
  website: authStore.user?.website || '',
  bio: authStore.user?.bio || '',
})

const rules: FormRules = {
  display_name: [{ required: true, message: '请输入昵称', trigger: 'blur' }],
}

const pwd = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

const pwdRules: FormRules = {
  old_password: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    {
      validator: (_rule: unknown, value: string, cb: (err?: Error) => void) => {
        const result = validatePasswordStrength(value)
        if (!result.valid) cb(new Error(result.message))
        else cb()
      },
      trigger: 'blur',
    },
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (_rule, value, cb) => {
        if (value !== pwd.new_password) cb(new Error('两次输入的密码不一致'))
        else cb()
      },
      trigger: 'blur',
    },
  ],
}

async function onSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    updateProfile.mutate(
      {
        display_name: form.display_name,
        email: form.email,
        avatar: form.avatar,
        website: form.website,
        bio: form.bio,
      },
      {
        onSuccess: () => ElMessage.success('资料已更新'),
        onError: (err) => ElMessage.error(getApiError(err, '保存失败')),
      },
    )
  })
}

async function onChangePassword() {
  if (!pwdFormRef.value) return
  await pwdFormRef.value.validate(async (valid) => {
    if (!valid) return
    changePassword.mutate(
      {
        old_password: pwd.old_password,
        new_password: pwd.new_password,
      },
      {
        onSuccess: () => {
          ElMessage.success('密码已修改')
          pwd.old_password = ''
          pwd.new_password = ''
          pwd.confirm_password = ''
        },
        onError: (err) => ElMessage.error(getApiError(err, '密码修改失败')),
      },
    )
  })
}

// ─── Two-factor authentication (TOTP) ────────────────────────────────

const totpEnabled = ref(false)
const totpLoading = ref(false)
const totpSetup = ref<{ secret: string; otpauth_uri: string } | null>(null)
const totpCode = ref('')
const totpPassword = ref('')
const backupVisible = ref(false)
const backupCodes = ref<string[]>([])

async function fetchTotpStatus() {
  try {
    totpEnabled.value = (await totpApi.status()).data.enabled
  } catch {
    // Status is cosmetic; ignore fetch failures.
  }
}

async function startTotpSetup() {
  totpLoading.value = true
  try {
    totpSetup.value = (await totpApi.setup()).data
    totpCode.value = ''
  } catch (err) {
    ElMessage.error(getApiError(err, '生成密钥失败'))
  } finally {
    totpLoading.value = false
  }
}

async function confirmTotpEnable() {
  if (totpCode.value.length !== 6) {
    ElMessage.warning('请输入 6 位验证码')
    return
  }
  totpLoading.value = true
  try {
    backupCodes.value = (await totpApi.enable(totpCode.value)).data.backup_codes
    totpEnabled.value = true
    totpSetup.value = null
    backupVisible.value = true
    ElMessage.success('两步验证已启用')
  } catch (err) {
    ElMessage.error(getApiError(err, '验证码错误或已过期'))
  } finally {
    totpLoading.value = false
  }
}

async function disableTotp() {
  if (!totpPassword.value) {
    ElMessage.warning('请输入账号密码')
    return
  }
  totpLoading.value = true
  try {
    await totpApi.disable(totpPassword.value)
    totpEnabled.value = false
    totpPassword.value = ''
    ElMessage.success('两步验证已解除')
  } catch (err) {
    ElMessage.error(getApiError(err, '密码验证失败'))
  } finally {
    totpLoading.value = false
  }
}

async function copyText(text: string) {
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}

onMounted(fetchTotpStatus)
</script>

<style lang="scss" scoped>
.profile-page {
  h2 {
    margin: 0 0 20px;
  }
}

.profile-summary {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 8px 0;

  h3 {
    margin: 8px 0 0;
  }

  .meta {
    color: #909399;
    font-size: 13px;
    margin: 0;
  }
}

.password-card {
  margin-top: 20px;
}

.totp-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.totp-desc {
  color: #909399;
  font-size: 13px;
  margin: 0 0 12px;
}

.totp-secret {
  margin: 12px 0;
  font-size: 13px;

  code {
    word-break: break-all;
    background: var(--el-fill-color-light);
    padding: 2px 6px;
    border-radius: 4px;
  }

  .totp-uri {
    margin-top: 8px;
  }
}

.totp-confirm {
  display: flex;
  gap: 8px;
  align-items: center;
}

.backup-codes {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  margin-top: 16px;

  code {
    background: var(--el-fill-color-light);
    padding: 6px 10px;
    border-radius: 4px;
    text-align: center;
  }
}
</style>
