import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'NRLLink 无线电互联系统',
        short_name: 'NRLLink',
        theme_color: '#0a0e14',
        icons: []
      }
    })
  ],
  server: {
    proxy: {
      '/api': 'http://localhost:9000',
      '/ws': { target: 'ws://localhost:9000', ws: true }
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: false
  }
})
