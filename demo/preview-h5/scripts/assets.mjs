import { cp, mkdir, readdir, rm, stat } from 'node:fs/promises'
import { resolve } from 'node:path'

const command = process.argv[2]
const root = resolve(import.meta.dirname, '..')
if (command === 'prepare') {
  const output = resolve(root, 'public/pdfjs')
  const source = resolve(root, 'node_modules/pdfjs-dist')
  await mkdir(output, { recursive: true })
  await cp(resolve(source, 'legacy/build/pdf.worker.min.mjs'), resolve(output, 'pdf.worker.min.mjs'))
  await cp(resolve(source, 'LICENSE'), resolve(output, 'LICENSE.txt'))
  for (const name of ['cmaps', 'standard_fonts', 'wasm']) await cp(resolve(source, name), resolve(output, name), { recursive: true })
} else if (command === 'copy') {
  const source = resolve(root, '.output/public')
  const output = resolve(root, '../../internal/module/preview/interfaces/reader-assets')
  await stat(resolve(source, 'index.html'))
  await mkdir(output, { recursive: true })
  for (const name of await readdir(output)) if (name !== 'README.md') await rm(resolve(output, name), { recursive: true, force: true })
  await cp(source, output, { recursive: true })
} else {
  throw new Error('Expected prepare or copy')
}
