import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import NotFound from './NotFound.vue'

describe('NotFound view', () => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div/>' } }],
  })

  const wrapper = mount(NotFound, {
    global: {
      plugins: [router],
      stubs: { 'el-button': { template: '<button><slot/></button>' } },
    },
  })

  it('renders 404 heading and message', () => {
    expect(wrapper.find('h1').text()).toBe('404')
    expect(wrapper.find('p').text()).toBe('页面不存在')
  })

  it('has a return-home button', () => {
    expect(wrapper.find('button').exists()).toBe(true)
    expect(wrapper.find('button').text()).toContain('返回首页')
  })
})
