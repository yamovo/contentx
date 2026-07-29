// defineConfig from vitest/config so the `test` block is properly typed.
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { resolve } from 'path'

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      imports: ['vue', 'vue-router', 'pinia'],
      resolvers: [ElementPlusResolver()],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: 'src/components.d.ts',
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'json-summary'],
      reportsDirectory: './coverage',
      // Ratcheted thresholds (Round 7 / A-13): set below current coverage to
      // prevent regression while tolerating env fluctuations (CI V8 may report
      // function coverage ~2pts lower than local).
      // Local: lines 44.32%, branches 87.43%, functions 41.78%, statements 44.32%
      // CI:     lines 45.3%,  branches 88.16%, functions 39.79%
      thresholds: {
        lines: 35,
        branches: 70,
        functions: 35,
        statements: 35,
      },
    },
    server: {
      deps: {
        inline: [/element-plus/],
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    // ECharts is lazy-loaded via defineAsyncComponent and ships as a single
    // ~515 KB chunk (175 KB gzip). Raising the limit silences the warning
    // without hiding real regressions (the next-largest chunk is ~232 KB).
    chunkSizeWarningLimit: 550,
    rollupOptions: {
      output: {
        // IMPORTANT: do NOT assign element-plus (or other barrel-export
        // libraries) to a manual chunk. Doing so marks the barrel module as a
        // chunk entry and Rollup then keeps ALL of its exports (~940KB min),
        // silently disabling tree-shaking (verified empirically: with the
        // manual chunk the bundle contained ElCarousel/ElTransfer/etc.;
        // without it they are shaken away and Element Plus code splits
        // naturally across route chunks). echarts is already isolated by the
        // route-level dynamic imports. Only the always-fully-used vue core
        // stack is grouped for long-term caching.
        manualChunks(id: string) {
          if (!id.includes('node_modules')) return
          if (
            id.includes('/vue/') || id.includes('@vue/') ||
            id.includes('vue-router') || id.includes('pinia') || id.includes('axios')
          ) return 'vendor'
        },
      },
    },
  },
})
