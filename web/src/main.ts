import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import {
  ArrowLeft, Brush, ChatDotSquare, Connection, DataAnalysis, Document,
  EditPen, Expand, Fold, Folder, Lightning, Lock, Menu, Moon, Notebook,
  Odometer, Picture, Plus, PriceTag, Rank, Search, Setting, StarFilled,
  Sunny, SwitchButton, Tickets, TrendCharts, Upload, User, View,
} from '@element-plus/icons-vue'
import App from './App.vue'
import router from './router'
import './assets/main.scss'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(ElementPlus, { locale: zhCn, size: 'default' })

// Register only the icons actually used in the app (route meta, dynamic
// components, and template tags) instead of the full ~280-icon set.
// This preserves tree-shaking and reduces bundle size.
const icons = {
  ArrowLeft, Brush, ChatDotSquare, Connection, DataAnalysis, Document,
  EditPen, Expand, Fold, Folder, Lightning, Lock, Menu, Moon, Notebook,
  Odometer, Picture, Plus, PriceTag, Rank, Search, Setting, StarFilled,
  Sunny, SwitchButton, Tickets, TrendCharts, Upload, User, View,
}
for (const [name, component] of Object.entries(icons)) {
  app.component(name, component)
}

app.mount('#app')
