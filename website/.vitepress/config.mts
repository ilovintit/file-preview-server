import { defineConfig } from 'vitepress'

const repo = 'https://github.com/ilovintit/file-preview-server'

export default defineConfig({
  lang: 'zh-CN',
  title: 'file-preview-server',
  description: '签名受控的文件在线预览服务：图片、PDF 与 Office/WPS 文档',
  base: '/file-preview-server/',
  cleanUrls: true,
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: '指南', link: '/guide/getting-started' },
      { text: '更新日志', link: `${repo}/releases` },
    ],
    sidebar: [
      {
        text: '指南',
        items: [
          { text: '快速开始', link: '/guide/getting-started' },
          { text: '配置', link: '/guide/configuration' },
          { text: '业务系统接入', link: '/guide/integration' },
          { text: '支持的格式', link: '/guide/formats' },
          { text: 'Kubernetes 部署', link: '/guide/kubernetes' },
        ],
      },
    ],
    socialLinks: [{ icon: 'github', link: repo }],
    editLink: {
      pattern: `${repo}/edit/main/website/:path`,
      text: '在 GitHub 上编辑此页',
    },
    search: { provider: 'local' },
    footer: {
      message: '基于 Apache-2.0 许可发布',
    },
  },
})
