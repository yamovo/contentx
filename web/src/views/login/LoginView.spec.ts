import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

// Hoisted auth store mock so the factory can reference it. The factory runs
// before any imports, so vi.hoisted is required.
const { mockAuthStore } = vi.hoisted(() => ({
  mockAuthStore: {
    login: vi.fn(),
    loading: false,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => mockAuthStore,
}))

vi.mock('animejs', () => ({
  animate: vi.fn(),
  createTimeline: vi.fn(),
}))

vi.mock('animejs/utils', () => ({
  stagger: vi.fn(),
}))

// Mock @/router defensively (LoginView uses useRouter() from vue-router, not
// @/router, but the mock is harmless and satisfies the task requirement).
vi.mock('@/router', () => ({
  default: { push: vi.fn() },
}))

vi.mock('element-plus', async (importOriginal) => {
  const actual = await importOriginal<typeof import('element-plus')>()
  const { mockElementPlus } = await import('@/test/utils')
  return { ...actual, ...mockElementPlus() }
})

// Note: @element-plus/icons-vue is NOT mocked — real SVG icons render in jsdom.

import { mountWithPlugins } from '@/test/utils'
import { ElMessage } from 'element-plus'
import LoginView from './LoginView.vue'

// Custom el-form stub that exposes a controllable validate() method, since
// LoginView calls formRef.value?.validate() and the default stub doesn't
// expose one.
const formValidate = vi.fn().mockResolvedValue(true)
const customFormStub = {
  template: '<form @submit.prevent="$emit(\'submit\')"><slot/></form>',
  methods: {
    validate: formValidate,
  },
}

describe('LoginView', () => {
  let routerPushSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.clearAllMocks()
    mockAuthStore.loading = false
    formValidate.mockResolvedValue(true)
  })

  async function mountLogin() {
    const wrapper = mountWithPlugins(LoginView, {
      global: { stubs: { 'el-form': customFormStub } },
    })
    await flushPromises()
    // useRouter() inside LoginView returns the router installed by
    // mountWithPlugins. Spy on its push method to assert redirects.
    const router = (wrapper.vm as any).$router
    routerPushSpy = vi.spyOn(router, 'push')
    return wrapper
  }

  it('renders the login form with logo and login button', async () => {
    const wrapper = await mountLogin()

    expect(wrapper.find('.login-card').exists()).toBe(true)
    expect(wrapper.text()).toContain('ContentX')
    expect(wrapper.text()).toContain('内容管理系统')
    // Login button exists.
    const loginBtn = wrapper.findAll('button').find((b) => b.text().includes('登'))
    expect(loginBtn).toBeTruthy()
  })

  it('does not call authStore.login when form validation fails', async () => {
    formValidate.mockResolvedValue(false)
    const wrapper = await mountLogin()

    const loginBtn = wrapper.findAll('button').find((b) => b.text().includes('登'))
    expect(loginBtn).toBeTruthy()
    await loginBtn!.trigger('click')
    await flushPromises()

    expect(formValidate).toHaveBeenCalled()
    expect(mockAuthStore.login).not.toHaveBeenCalled()
  })

  it('calls authStore.login and redirects on successful login', async () => {
    mockAuthStore.login.mockResolvedValueOnce({})
    const wrapper = await mountLogin()

    // Fill in the username and password inputs.
    const inputs = wrapper.findAll('input')
    const usernameInput = inputs.find((i) => i.attributes('placeholder') === '用户名或邮箱')
    const passwordInput = inputs.find((i) => i.attributes('placeholder') === '密码')
    expect(usernameInput).toBeTruthy()
    expect(passwordInput).toBeTruthy()

    await usernameInput!.setValue('testuser')
    await passwordInput!.setValue('password123')
    await flushPromises()

    const loginBtn = wrapper.findAll('button').find((b) => b.text().includes('登'))
    await loginBtn!.trigger('click')
    await flushPromises()

    expect(formValidate).toHaveBeenCalled()
    expect(mockAuthStore.login).toHaveBeenCalledWith('testuser', 'password123')
    expect(ElMessage.success).toHaveBeenCalledWith('登录成功')
    expect(routerPushSpy).toHaveBeenCalledWith('/admin')
  })

  it('shows ElMessage.error when login rejects', async () => {
    mockAuthStore.login.mockRejectedValueOnce(new Error('Invalid credentials'))
    const wrapper = await mountLogin()

    const inputs = wrapper.findAll('input')
    const usernameInput = inputs.find((i) => i.attributes('placeholder') === '用户名或邮箱')
    const passwordInput = inputs.find((i) => i.attributes('placeholder') === '密码')
    await usernameInput!.setValue('baduser')
    await passwordInput!.setValue('badpass')
    await flushPromises()

    const loginBtn = wrapper.findAll('button').find((b) => b.text().includes('登'))
    await loginBtn!.trigger('click')
    await flushPromises()

    expect(mockAuthStore.login).toHaveBeenCalled()
    expect(ElMessage.error).toHaveBeenCalledWith('登录失败')
  })

  it('uses redirect query when present', async () => {
    mockAuthStore.login.mockResolvedValueOnce({})
    const wrapper = await mountLogin()

    // Navigate to a route with ?redirect=/dashboard so useRoute() picks it up.
    const router = (wrapper.vm as any).$router
    await router.push({ path: '/', query: { redirect: '/dashboard' } })
    await flushPromises()

    const inputs = wrapper.findAll('input')
    const usernameInput = inputs.find((i) => i.attributes('placeholder') === '用户名或邮箱')
    const passwordInput = inputs.find((i) => i.attributes('placeholder') === '密码')
    await usernameInput!.setValue('testuser')
    await passwordInput!.setValue('password123')
    await flushPromises()

    const loginBtn = wrapper.findAll('button').find((b) => b.text().includes('登'))
    await loginBtn!.trigger('click')
    await flushPromises()

    expect(routerPushSpy).toHaveBeenCalledWith('/dashboard')
  })
})
