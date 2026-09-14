import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import legacy from '@vitejs/plugin-legacy'
import { readFileSync } from 'fs'

const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf-8'))

function formatBuildTime() {
  const d = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// 旧浏览器兼容基线：支持 ES2015 但无原生 ESM 的浏览器。
// 现代浏览器加载原生 ESM 包，旧浏览器通过 <script nomodule> 加载转译后的 legacy 包。
// 注：Vue 3 响应式依赖 Proxy（无法 polyfill），因此不支持 IE11。
const legacyTargets = ['chrome >= 51', 'firefox >= 54', 'safari >= 10', 'edge >= 15']

export default defineConfig({
  plugins: [
    vue(),
    legacy({
      targets: legacyTargets,
      // 现代包基线为 chrome64，同样按需注入 core-js polyfill（如 Array.prototype.at）
      modernPolyfills: true
    })
  ],
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
    __BUILD_TIME__: JSON.stringify(formatBuildTime())
  },
  server: {
    port: 5173,
    proxy: {
      '/api/': 'http://localhost:9868'
    }
  },
  build: {
    // 让 esbuild 按旧浏览器降级 CSS 新语法
    cssTarget: 'chrome61',
    rollupOptions: {
      output: {
        manualChunks: {
          vue: ['vue', 'vue-router'],
          'element-plus': ['element-plus']
        }
      }
    }
  }
})
