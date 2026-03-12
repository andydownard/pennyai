// =============================================================================
// Vite Configuration
//
// Vite is the build tool that:
//   - Compiles TypeScript → JavaScript
//   - Bundles React into optimized files
//   - Provides hot-reload during development
//
// In development: "npm run dev" starts Vite on port 5173 and proxies
// /auth/* requests to the Go server on port 8080.
//
// In production: "npm run build" outputs static files to ../static/
// which the Go server serves directly.
// =============================================================================

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],

  // Output the built files to ../static/ so Go can serve them
  build: {
    outDir: '../static',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          // Separate React into its own cacheable vendor chunk
          vendor: ['react', 'react-dom'],
          // Split observability libraries — not needed for initial render
          observability: ['@sentry/react', 'posthog-js'],
        },
      },
    },
  },

  // During local development, proxy API calls to the Go backend
  server: {
    host: true,
    proxy: {
      '/auth': 'http://localhost:8080',
      '/api': 'http://localhost:8080',
    },
  },

  // Test configuration for vitest
  // @ts-expect-error vitest/config extends vite config
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
})
