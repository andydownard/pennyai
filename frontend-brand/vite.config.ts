import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      '/brand': 'http://localhost:8080',
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: '../static-brand',
  },
})
