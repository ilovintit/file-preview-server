<script setup lang="ts">
import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask } from 'pdfjs-dist'

type Phase = 'loading' | 'ready' | 'expired' | 'invalid' | 'error'
const phase = ref<Phase>('loading')
const filename = ref('文件预览')
const kind = ref<'image' | 'pdf'>('pdf')
const page = ref(1)
const pages = ref(0)
const zoom = ref(100)
const busy = ref(false)
const imageURL = ref('')
const imageWidth = ref(0)
const pageText = ref('')
const generation = ref(0)
const viewport = ref<HTMLElement>()
const canvas = ref<HTMLCanvasElement>()
const image = ref<HTMLImageElement>()
let token = ''
let expiresAt = 0
let loading: PDFDocumentLoadingTask | undefined
let pdfDocument: PDFDocumentProxy | undefined
let rendering: RenderTask | undefined
let loadingGeneration = 0
let resizeTimer: ReturnType<typeof setTimeout> | undefined
let renderGeneration = 0
let timer: ReturnType<typeof setTimeout> | undefined
let diagnosis: AbortController | undefined

const message = computed(() => ({
  loading: '正在准备预览',
  ready: '文件已加载',
  expired: '预览链接已失效',
  invalid: '无法预览此文件',
  error: '文件加载失败'
}[phase.value]))
const detail = computed(() => ({
  loading: '文件较大或需要转换时，请稍候。',
  ready: '',
  expired: '请返回附件页面重新获取预览。',
  invalid: '文件内容、格式或页面尺寸不符合预览要求。',
  error: '网络或服务暂时不可用，请重试或返回。'
}[phase.value]))

function clearDocument() {
  renderGeneration++
  rendering?.cancel()
  rendering = undefined
  if (loading) {
    const task = loading
    const owner = loadingGeneration
    void task.destroy().catch(() => {
      if (owner === generation.value && phase.value === 'loading') { busy.value = false; phase.value = 'error' }
    })
  }
  loading = undefined
  pdfDocument = undefined
  image.value?.removeAttribute('src')
  imageURL.value = ''
  imageWidth.value = 0
  pageText.value = ''
  if (canvas.value) { canvas.value.width = 0; canvas.value.height = 0 }
  diagnosis?.abort()
  diagnosis = undefined
  busy.value = false
}

function expire() {
  generation.value++
  clearDocument()
  token = ''
  phase.value = 'expired'
  if (timer) clearTimeout(timer)
}

async function classifyFailure(current: number) {
  if (current !== generation.value) return
  phase.value = 'error'
  clearDocument()
  busy.value = false
  diagnosis?.abort()
  const controller = new AbortController()
  diagnosis = controller
  try {
    const response = await fetch(`/v/${token}`, { redirect: 'manual', cache: 'no-store', signal: controller.signal })
    if (current !== generation.value) return
    if (response.status === 404) expire()
    else if (response.status === 422) phase.value = 'invalid'
  } catch {
    if (current === generation.value && !controller.signal.aborted) phase.value = 'error'
  }
}

async function renderPage(current: number) {
  const request = ++renderGeneration
  const previous = rendering
  previous?.cancel()
  busy.value = true
  try {
    if (previous) {
      try { await previous.promise } catch (error) {
        if ((error as Error).name !== 'RenderingCancelledException') throw error
      }
    }
    if (!pdfDocument || current !== generation.value || request !== renderGeneration) return
    const pdfPage = await pdfDocument.getPage(page.value)
    await nextTick()
    if (!canvas.value || !viewport.value || current !== generation.value || request !== renderGeneration) return
    const natural = pdfPage.getViewport({ scale: 1 })
    const available = Math.max(160, viewport.value.clientWidth - 32)
    const scale = Math.min(available / natural.width, 1.8) * zoom.value / 100
    const visual = pdfPage.getViewport({ scale })
    const ratio = Math.min(window.devicePixelRatio || 1, 2, Math.sqrt(16000000 / (visual.width * visual.height)), 16384 / Math.max(visual.width, visual.height))
    if (!Number.isFinite(ratio) || ratio < 0.1 || visual.width <= 0 || visual.height <= 0) {
      phase.value = 'invalid'; busy.value = false; return
    }
    canvas.value.width = Math.ceil(visual.width * ratio)
    canvas.value.height = Math.ceil(visual.height * ratio)
    canvas.value.style.width = `${visual.width}px`
    canvas.value.style.height = `${visual.height}px`
    rendering = pdfPage.render({ canvas: canvas.value, viewport: visual, transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0] })
    await rendering.promise
    const text = await pdfPage.getTextContent()
    if (current === generation.value && request === renderGeneration) {
      pageText.value = text.items.map(item => 'str' in item ? item.str : '').join(' ')
      busy.value = false
      phase.value = 'ready'
    }
  } catch (error) {
    if ((error as Error).name !== 'RenderingCancelledException' && current === generation.value && request === renderGeneration) await classifyFailure(current)
  }
}

async function start() {
  const current = ++generation.value
  clearDocument()
  if (!token || expiresAt * 1000 <= Date.now()) { expire(); return }
  phase.value = 'loading'
  page.value = 1
  pages.value = 0
  zoom.value = 100
  await nextTick()
  if (kind.value === 'image') { imageURL.value = `/v/${token}`; return }
  try {
    const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs')
    if (current !== generation.value) return
    pdfjs.GlobalWorkerOptions.workerSrc = '/reader/pdfjs/pdf.worker.min.mjs'
    const task = pdfjs.getDocument({
      url: `/v/${token}`, withCredentials: false, verbosity: 0,
      cMapUrl: '/reader/pdfjs/cmaps/', cMapPacked: true,
      standardFontDataUrl: '/reader/pdfjs/standard_fonts/',
      wasmUrl: '/reader/pdfjs/wasm/', maxImageSize: 40000000,
      enableXfa: false
    })
    loading = task
    loadingGeneration = current
    const pdf = await task.promise
    if (current !== generation.value) { await task.destroy(); return }
    pdfDocument = pdf
    pages.value = pdf.numPages
    await renderPage(current)
  } catch {
    if (current === generation.value) await classifyFailure(current)
  }
}

function imageLoaded(event: Event) {
  const target = event.target as HTMLImageElement
  if (Number(target.dataset.generation) === generation.value && phase.value === 'loading') { updateImageWidth(); phase.value = 'ready' }
}

function imageFailed(event: Event) {
  const target = event.target as HTMLImageElement
  if (Number(target.dataset.generation) === generation.value && (phase.value === 'loading' || phase.value === 'ready')) void classifyFailure(generation.value)
}

function turnPage(delta: number) {
  if (busy.value || phase.value !== 'ready') return
  page.value = Math.max(1, Math.min(pages.value, page.value + delta))
  void renderPage(generation.value)
}

function changeZoom(value: number) {
  if (busy.value || phase.value !== 'ready') return
  zoom.value = Math.max(75, Math.min(175, value))
  if (kind.value === 'pdf') void renderPage(generation.value)
  else updateImageWidth()
}

function back() {
  expire()
  if (window.history.length > 1) window.history.back()
  else window.close()
}

function updateImageWidth() {
  const available = Math.max(160, (viewport.value?.clientWidth || 320) - 32)
  imageWidth.value = Math.min(image.value?.naturalWidth || available, available) * zoom.value / 100
}

function resized() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => { if (phase.value === 'ready') { if (kind.value === 'pdf') void renderPage(generation.value); else updateImageWidth() } }, 120)
}

function restored(event: PageTransitionEvent) { if (event.persisted) expire() }

onMounted(() => {
  const fragment = new URLSearchParams(window.location.hash.slice(1))
  window.history.replaceState(null, '', window.location.pathname)
  const value = fragment.get('token') || ''
  const type = fragment.get('preview_type')
  const expiry = Number(fragment.get('expires_at'))
  if (!/^[0-9a-f]{32}$/.test(value) || (type !== 'image' && type !== 'pdf') || !Number.isSafeInteger(expiry) || expiry * 1000 <= Date.now()) {
    phase.value = 'expired'
    return
  }
  token = value
  kind.value = type
  filename.value = (fragment.get('filename') || '文件预览').slice(0, 512)
  expiresAt = expiry
  timer = setTimeout(expire, Math.min(expiry * 1000 - Date.now(), 86400000))
  window.addEventListener('resize', resized)
  window.addEventListener('pagehide', expire)
  window.addEventListener('pageshow', restored)
  void start()
})

onBeforeUnmount(() => {
  expire()
  if (resizeTimer) clearTimeout(resizeTimer)
  window.removeEventListener('resize', resized)
  window.removeEventListener('pagehide', expire)
  window.removeEventListener('pageshow', restored)
})
</script>

<template>
  <main class="reader-app" :data-phase="phase" :data-kind="kind">
    <header class="reader-header">
      <button id="reader-back" class="text-button" type="button" @click="back">‹ 返回</button>
      <h1 :title="filename">{{ filename }}</h1>
      <span v-if="phase === 'ready'" class="file-tag">{{ kind === 'image' ? '图片' : 'PDF' }}</span>
    </header>
    <nav v-if="phase === 'ready'" class="reader-toolbar" aria-label="阅读工具">
      <div v-if="kind === 'pdf'" class="tool-group">
        <button id="previous-page" type="button" aria-label="上一页" :disabled="busy || page <= 1" @click="turnPage(-1)">‹</button>
        <span id="page-counter" aria-live="polite">{{ page }} / {{ pages }}</span>
        <button id="next-page" type="button" aria-label="下一页" :disabled="busy || page >= pages" @click="turnPage(1)">›</button>
      </div>
      <div class="tool-group">
        <button id="zoom-out" type="button" aria-label="缩小" :disabled="busy || zoom <= 75" @click="changeZoom(zoom - 25)">−</button>
        <span id="zoom-value">{{ zoom }}%</span>
        <button id="zoom-in" type="button" aria-label="放大" :disabled="busy || zoom >= 175" @click="changeZoom(zoom + 25)">+</button>
      </div>
      <button id="fit-width" class="fit-button" type="button" :disabled="busy" @click="changeZoom(100)">适合宽度</button>
    </nav>
    <section ref="viewport" class="reader-viewport" aria-label="文件内容" :aria-busy="phase === 'loading' || busy">
      <div v-if="phase === 'loading' || phase === 'ready'" class="document-area" :class="{ preparing: phase === 'loading' }">
        <img v-if="kind === 'image'" id="reader-image" :key="generation" ref="image" :src="imageURL || undefined" :data-generation="generation" :alt="filename" :style="{ width: imageWidth ? `${imageWidth}px` : undefined }" @load="imageLoaded" @error="imageFailed">
        <canvas v-else id="reader-canvas" ref="canvas" :aria-label="`文档第 ${page} 页`" />
      </div>
      <div v-if="phase !== 'ready'" id="reader-feedback" class="reader-feedback" role="status" aria-live="polite">
        <span v-if="phase === 'loading'" class="spinner" aria-hidden="true" />
        <span v-else class="feedback-mark" aria-hidden="true">!</span>
        <h2>{{ message }}</h2>
        <p>{{ detail }}</p>
        <div class="feedback-actions">
          <button v-if="phase === 'error'" id="retry-preview" class="primary-button" type="button" @click="start">重新加载</button>
          <button class="secondary-button" type="button" @click="back">返回附件页面</button>
        </div>
      </div>
    <div id="reader-text" class="sr-only" aria-label="当前页文字">{{ pageText }}</div>
    </section>
    <footer class="reader-footer"><span id="reader-status" aria-live="polite">{{ busy ? '正在呈现页面' : message }}</span><span>只读预览</span></footer>
  </main>
</template>
