import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // When VITE_API_BASE_URL is empty, frontend calls use these relative routes
    // and Vite proxies them to the local Go backend on :8080.
    // When VITE_API_BASE_URL is set, calls go directly to that API origin.
    proxy: {
      '/health':    { target: 'http://localhost:8080', changeOrigin: true },
      '/platforms': { target: 'http://localhost:8080', changeOrigin: true },
      '/oauth':     { target: 'http://localhost:8080', changeOrigin: true },
      '/batches':   { target: 'http://localhost:8080', changeOrigin: true },
      '/jobs':      { target: 'http://localhost:8080', changeOrigin: true },
      '/api':       { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
