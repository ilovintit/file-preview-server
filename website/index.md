---
layout: home

hero:
  name: file-preview-server
  text: 签名受控的文件在线预览
  tagline: 业务后端签发短期 token，浏览器只拿到预览链接，永远看不到源文件地址与存储凭据。
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: GitHub
      link: https://github.com/ilovintit/file-preview-server

features:
  - title: 18 种常见格式
    details: 常见图片、PDF、微软 Word/Excel/PowerPoint 新旧版与金山 WPS 文件；Office 与 BMP/TIFF 转为 PDF 阅读。
  - title: HMAC 签名与防重放
    details: 签发与管理接口只接受 HTTPS 上的 HMAC-SHA256 签名请求，nonce 原子占用，支持双密钥轮换。
  - title: 内容 hash 转换缓存
    details: 以内容 SHA-256 与转换器版本为身份缓存产物，按 token 与缓存双 TTL 自动清理，Valkey 管理生命周期。
  - title: 可替换对象存储
    details: 内置阿里云 OSS 与 S3 兼容（silo/MinIO）两种存储 profile，预览以短时签名 URL 302 跳转。
---
