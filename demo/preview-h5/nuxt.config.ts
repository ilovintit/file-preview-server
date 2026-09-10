export default defineNuxtConfig({
  ssr: false,
  compatibilityDate: '2026-09-01',
  devtools: { enabled: false },
  telemetry: false,
  modules: ['@nuxtjs/tailwindcss'],
  tailwindcss: { cssPath: '~/assets/main.css', viewer: false },
  app: {
    baseURL: '/reader/',
    head: {
      title: '文件预览',
      htmlAttrs: { lang: 'zh-CN' },
      meta: [{ name: 'referrer', content: 'no-referrer' }]
    }
  }
})
