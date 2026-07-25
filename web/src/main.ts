import { createApp } from 'vue'
import { createPinia } from 'pinia'
// Element Plus components are auto-imported on demand via
// unplugin-vue-components (ElementPlusResolver) — no full bundle import here.
// Only styles that the resolver cannot see are added manually:
// - dark theme CSS variables (global)
// - styles for imperatively-called APIs (ElMessage / ElMessageBox / v-loading)
import 'element-plus/theme-chalk/dark/css-vars.css'
import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/el-message.css'
import 'element-plus/theme-chalk/el-message-box.css'
import 'element-plus/theme-chalk/el-overlay.css'
import 'element-plus/theme-chalk/el-loading.css'
import {
  ArrowDown, ArrowLeft, Bottom, Brush, ChatDotSquare, Connection,
  DataAnalysis, Document, EditPen, Expand, Fold, Folder, Lightning, Lock,
  Menu, Moon, Notebook, Odometer, Picture, Plus, PriceTag, Rank, Search,
  Setting, StarFilled, Sunny, SwitchButton, Tickets, Top, TrendCharts,
  Upload, User, View,
} from '@element-plus/icons-vue'
import App from './App.vue'
import router from './router'
import './assets/main.scss'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)

// Register only the icons actually used in the app (route meta, dynamic
// components, and template tags) instead of the full ~280-icon set.
// This preserves tree-shaking and reduces bundle size.
const icons = {
  ArrowDown, ArrowLeft, Bottom, Brush, ChatDotSquare, Connection,
  DataAnalysis, Document, EditPen, Expand, Fold, Folder, Lightning, Lock,
  Menu, Moon, Notebook, Odometer, Picture, Plus, PriceTag, Rank, Search,
  Setting, StarFilled, Sunny, SwitchButton, Tickets, Top, TrendCharts,
  Upload, User, View,
}
for (const [name, component] of Object.entries(icons)) {
  app.component(name, component)
}

app.mount('#app')
