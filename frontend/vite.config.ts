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
          // Sentry + posthog are no longer force-chunked here: they are now
          // imported dynamically (main.tsx on idle, analytics.ts on first
          // event, ErrorBoundary on catch), so Rollup splits them into async
          // chunks that load off the critical path instead of being preloaded.
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
    // Only run unit/component tests under src/. The e2e/ specs use the
    // Playwright test runner (npm run test:e2e); without this, vitest's
    // default glob picks up e2e/*.spec.ts and crashes on Playwright's API.
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
})
