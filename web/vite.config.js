import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    // 原型需要在远程主机上访问：监听所有网卡（默认只绑 localhost）
    host: true,
    port: 5173,
    strictPort: true,
    // 若通过域名/反向代理访问，需加 allowedHosts: ['your.domain']（Vite 会校验 Host 头）
    proxy: {
      '/api': {
        target: 'http://localhost:8022',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
