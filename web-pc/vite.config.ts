import { fileURLToPath, URL } from 'node:url'
import { createLogger, defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 后端 API 未启动或正在重启时，代理连接会被拒绝（ECONNREFUSED/ECONNRESET），
// vite 默认会打印整段错误堆栈。这里只过滤掉这一类噪音，其余日志照常输出。
const logger = createLogger()
const logError = logger.error.bind(logger)
const silentCodes = ['ECONNREFUSED', 'ECONNRESET']
logger.error = (msg, options) => {
  const code = (options?.error as NodeJS.ErrnoException | undefined)?.code
  if ((code && silentCodes.includes(code)) || silentCodes.some(item => msg.includes(item))) return
  logError(msg, options)
}

export default defineConfig({
  base: '/pc/',
  customLogger: logger,
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)), '@shared': fileURLToPath(new URL('../packages/shared/src', import.meta.url)) } },
  server: { port: 5173, strictPort: true, proxy: { '/api': 'http://127.0.0.1:8090', '/health': 'http://127.0.0.1:8090' } }
})
