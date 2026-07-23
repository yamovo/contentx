import { mount, type ComponentMountingOptions } from '@vue/test-utils'
import {
  defineComponent,
  h,
  computed,
  provide,
  inject,
  type Component,
  type InjectionKey,
  type Ref,
} from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'

/**
 * Factory that builds a fresh localStorage mock backed by an in-memory store.
 * Eliminates the boilerplate duplicated in auth.spec.ts and app.spec.ts.
 */
export function createLocalStorageMock() {
  let store: Record<string, string> = {}
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = String(value)
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key]
    }),
    clear: vi.fn(() => {
      store = {}
    }),
    get length() {
      return Object.keys(store).length
    },
    key: vi.fn((index: number) => Object.keys(store)[index] || null),
  }
}

/**
 * Install a localStorage mock on globalThis. Returns the mock so callers can
 * reset/inspect it. Safe to call multiple times — each call installs a fresh
 * backing store.
 */
export function installLocalStorageMock() {
  const mock = createLocalStorageMock()
  Object.defineProperty(globalThis, 'localStorage', {
    value: mock,
    configurable: true,
  })
  return mock
}

/**
 * Simple stub templates for the Element Plus components used across the CMS
 * views. Stubs render their slots and forward common events so tests can
 * trigger clicks / input without pulling in the full EP runtime.
 */
const buttonStub = {
  template:
    '<button @click="$emit(\'click\', $event)"><slot/></button>',
}
const iconStub = { template: '<i><slot/></i>' }
const slotStub = { template: '<div><slot/></div>' }
const inputStub = {
  template:
    '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" @keyup.enter="$emit(\'keyup.enter\', $event)" />',
}
// Injection key for the smart el-table / el-table-column stubs. The table
// provides its `data` prop as a computed ref; each column injects it and
// renders its #default slot for every row, passing `{ row, $index }`.
const tableDataKey: InjectionKey<Ref<any[]>> = Symbol('el-table-data')
const tableStub = defineComponent({
  name: 'ElTable',
  props: { data: { type: Array, default: () => [] } },
  setup(props, { slots }) {
    provide(tableDataKey, computed(() => props.data) as Ref<any[]>)
    return () => h('div', { class: 'el-table-stub' }, slots.default?.())
  },
})
const tableColumnStub = defineComponent({
  name: 'ElTableColumn',
  props: {
    label: String,
    prop: String,
    width: [String, Number],
    align: String,
  },
  setup(props, { slots }) {
    const dataRef = inject(tableDataKey, computed(() => []))
    return () => {
      const rows = dataRef.value || []
      return h(
        'div',
        { class: 'el-table-column-stub' },
        rows.map((row: any, i: number) =>
          slots.default
            ? slots.default({ row, $index: i })
            : h('span', {}, props.prop ? String(row[props.prop] ?? '') : ''),
        ),
      )
    }
  },
})
const tagStub = { template: '<span><slot/></span>' }
const dialogStub = {
  template:
    '<div v-if="modelValue" class="el-dialog-stub"><div class="el-dialog__title">{{ title }}</div><slot/><slot name="footer"/></div>',
  props: { modelValue: Boolean, title: String },
}
const formStub = { template: '<form @submit.prevent="$emit(\'submit\')"><slot/></form>' }
const formItemStub = { template: '<div><slot/></div>' }
const colorPickerStub = {
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  props: { modelValue: String },
}
const switchStub = {
  template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
  props: { modelValue: Boolean },
}
const datePickerStub = slotStub
const tooltipStub = slotStub
const emptyStub = {
  template: '<div>{{ description }}<slot/></div>',
  props: { description: String },
}
const popconfirmStub = {
  template:
    '<span><slot name="reference" /><button class="popconfirm-confirm" @click="$emit(\'confirm\')">confirm</button></span>',
  emits: ['confirm'],
}
const dropdownStub = { template: '<div><slot/></div>' }
const dropdownMenuStub = { template: '<div><slot/></div>' }
const dropdownItemStub = {
  template: '<div @click="$emit(\'click\')"><slot/></div>',
}
const cardStub = { template: '<div><slot/></div>' }
const rowStub = { template: '<div><slot/></div>' }
const colStub = { template: '<div><slot/></div>' }
const paginationStub = slotStub
const loadingStub = slotStub
const imageStub = slotStub
const uploadStub = slotStub
const badgeStub = slotStub
const avatarStub = slotStub
const skeletonStub = slotStub
const dividerStub = slotStub
const linkStub = {
  template: '<a @click="$emit(\'click\')"><slot/></a>',
}
const tabsStub = slotStub
const tabPaneStub = slotStub
const radioGroupStub = slotStub
const radioStub = slotStub
const checkboxStub = {
  template:
    '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
  props: { modelValue: Boolean },
}
const breadcrumbStub = slotStub
const breadcrumbItemStub = slotStub
const statisticStub = slotStub
const progressStub = slotStub
const selectStub = {
  template:
    '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><slot/></select>',
  props: { modelValue: [String, Number, Object] },
}
const optionStub = { template: '<option><slot/></option>' }
const treeSelectStub = {
  template:
    '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><slot/></select>',
  props: { modelValue: [String, Number, Object], data: Array, props: Object },
}
const inputNumberStub = {
  template:
    '<input :value="modelValue" @input="$emit(\'update:modelValue\', Number($event.target.value))" />',
  props: { modelValue: Number, min: Number },
}

/**
 * Default Element Plus stubs map. Spread into `global.stubs` or extend via
 * `mountWithPlugins`.
 */
export const elementPlusStubs = {
  'el-button': buttonStub,
  'el-icon': iconStub,
  'el-table': tableStub,
  'el-table-column': tableColumnStub,
  'el-tag': tagStub,
  'el-dialog': dialogStub,
  'el-form': formStub,
  'el-form-item': formItemStub,
  'el-input': inputStub,
  'el-select': selectStub,
  'el-option': optionStub,
  'el-pagination': paginationStub,
  'el-popconfirm': popconfirmStub,
  'el-dropdown': dropdownStub,
  'el-dropdown-menu': dropdownMenuStub,
  'el-dropdown-item': dropdownItemStub,
  'el-card': cardStub,
  'el-row': rowStub,
  'el-col': colStub,
  'el-color-picker': colorPickerStub,
  'el-switch': switchStub,
  'el-date-picker': datePickerStub,
  'el-tooltip': tooltipStub,
  'el-loading': loadingStub,
  'el-empty': emptyStub,
  'el-image': imageStub,
  'el-upload': uploadStub,
  'el-badge': badgeStub,
  'el-avatar': avatarStub,
  'el-skeleton': skeletonStub,
  'el-divider': dividerStub,
  'el-link': linkStub,
  'el-tabs': tabsStub,
  'el-tab-pane': tabPaneStub,
  'el-radio-group': radioGroupStub,
  'el-radio': radioStub,
  'el-checkbox': checkboxStub,
  'el-breadcrumb': breadcrumbStub,
  'el-breadcrumb-item': breadcrumbItemStub,
  'el-statistic': statisticStub,
  'el-progress': progressStub,
  'el-tree-select': treeSelectStub,
  'el-input-number': inputNumberStub,
} as const

/**
 * Mount a component pre-wired with Pinia (active pinia set), a memory router,
 * and the common Element Plus stubs. Callers can override/extend any global
 * option (plugins, stubs, provide, etc.).
 *
 * Returns the wrapper. The installed router is accessible via
 * `wrapper.vm.$router`.
 */
export function mountWithPlugins(
  component: Component,
  options: ComponentMountingOptions<any> = {},
) {
  setActivePinia(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div/>' } }],
  })

  const { global: globalOverride, ...rest } = options
  // Destructure stubs and plugins out so the spread below doesn't clobber
  // the merged values.
  const {
    stubs: userStubs,
    plugins: userPlugins,
    ...restGlobal
  } = (globalOverride as any) || {}

  return mount(component, {
    global: {
      plugins: [router, ...(userPlugins || [])],
      stubs: { ...elementPlusStubs, ...(userStubs || {}) },
      ...restGlobal,
    },
    ...rest,
  } as ComponentMountingOptions<any>)
}

/**
 * Returns the Element Plus service mocks (ElMessage, ElMessageBox). Use as the
 * factory body for `vi.mock('element-plus', ...)`. Relies on `vi` being a
 * global (globals: true in vite.config.ts).
 */
export function mockElementPlus() {
  return {
    ElMessage: {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn(),
    },
    ElMessageBox: {
      confirm: vi.fn().mockResolvedValue('confirm'),
      alert: vi.fn().mockResolvedValue('confirm'),
      prompt: vi.fn().mockResolvedValue({ value: '', action: 'confirm' }),
    },
  }
}

/**
 * Returns the full @/api mock object covering every namespace exported by
 * src/api/index.ts. Each method is a vi.fn() with no default implementation —
 * tests can `.mockResolvedValueOnce` / `.mockRejectedValueOnce` per case.
 * Relies on `vi` being a global.
 */
export function mockApi() {
  return {
    authApi: {
      login: vi.fn(),
      register: vi.fn(),
      refresh: vi.fn(),
      logout: vi.fn(),
      me: vi.fn(),
      updateProfile: vi.fn(),
      changePassword: vi.fn(),
    },
    articleApi: {
      list: vi.fn(),
      get: vi.fn(),
      getBySlug: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      bulk: vi.fn(),
      revisions: vi.fn(),
      restoreRevision: vi.fn(),
      like: vi.fn(),
    },
    categoryApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      reorder: vi.fn(),
    },
    tagApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      merge: vi.fn(),
    },
    commentApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      approve: vi.fn(),
      spam: vi.fn(),
      trash: vi.fn(),
      bulk: vi.fn(),
      stats: vi.fn(),
      articleComments: vi.fn(),
    },
    mediaApi: {
      list: vi.fn(),
      get: vi.fn(),
      upload: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      bulkDelete: vi.fn(),
      folders: vi.fn(),
      stats: vi.fn(),
    },
    userApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      resetPassword: vi.fn(),
    },
    roleApi: {
      list: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      permissions: vi.fn(),
    },
    settingsApi: {
      list: vi.fn(),
      get: vi.fn(),
      update: vi.fn(),
      public: vi.fn(),
    },
    seoApi: {
      getSetting: vi.fn(),
      updateSetting: vi.fn(),
      sitemap: vi.fn(),
      robotsTxt: vi.fn(),
      listRedirects: vi.fn(),
      createRedirect: vi.fn(),
      deleteRedirect: vi.fn(),
    },
    menuApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      addItem: vi.fn(),
      updateItem: vi.fn(),
      deleteItem: vi.fn(),
      reorderItems: vi.fn(),
    },
    analyticsApi: {
      dashboard: vi.fn(),
      viewsOverTime: vi.fn(),
      topReferrers: vi.fn(),
      deviceBreakdown: vi.fn(),
      recordView: vi.fn(),
    },
    pluginApi: {
      list: vi.fn(),
      enable: vi.fn(),
      disable: vi.fn(),
      updateConfig: vi.fn(),
    },
    themeApi: {
      list: vi.fn(),
      activate: vi.fn(),
      updateConfig: vi.fn(),
    },
    systemApi: {
      info: vi.fn(),
      health: vi.fn(),
      activity: vi.fn(),
    },
  }
}
