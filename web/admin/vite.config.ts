import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiBase = env.VITE_API_BASE_URL ?? 'http://localhost:9110'

  return {
    plugins: [react()],
    server: {
      proxy: {
        '/admin': {
          target: apiBase,
          changeOrigin: true,
        },
      },
    },
  }
})
