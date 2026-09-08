// Vendor explicitly selected, version-pinned upstream regression fixtures.
// Never execute upstream scripts; verify each original Git blob before writing.
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const root = resolve('internal/module/preview/testdata/text');
const lo = 'https://raw.githubusercontent.com/LibreOffice/core/11ccf4230941895f97e4fb6987631b65b401858b/';
const tika = 'https://raw.githubusercontent.com/apache/tika/e918be5e9d1031a373528215a32ea5a1e6488624/';
const mime = 'https://gitlab.freedesktop.org/xdg/shared-mime-info/-/raw/dd8998f4edb0cf06afe2fc199e6ec0c8fda6e756/';
const writer = 'writerperfect/qa/unit/data/writer/';
const files = [
  ['wpd', tika, 'tika-parsers/tika-parsers-standard/tika-parsers-standard-modules/tika-parser-miscoffice-module/src/test/resources/test-documents/testWordPerfect.wpd', 'd577b1c1759f3d0594272dcd91ae95cf1adbeaf3', 'Apache-2.0'],
  ['pages', lo, writer + 'libetonyek/pass/Pages_4.pages', '43c9213922bd26f9242ae93ed6a5aabcc85df11f', 'MPL-2.0'],
  ['cwk', lo, writer + 'libmwaw/pass/ClarisWorks_6.0.cwk', '9162e3d2c69ec42d5f6bd53a00fecac2d9f07d07', 'MPL-2.0'],
  ['mw', lo, writer + 'libmwaw/pass/MacWrite_4.5', '725f9ad4fd0a79ebac0b19232cf3ac8a57f38e7f', 'MPL-2.0'],
  ['mcw', lo, writer + 'libmwaw/pass/MacWrite_Pro1.0', '82e5d860848c2056e425bd7bdda6a7dbd1e384e7', 'MPL-2.0'],
  ['psw', lo, writer + 'libwps/pass/PocketWord.psw', '75c0b60f397cfa1ceb96059abab2bf98fa890bea', 'MPL-2.0'],
  ['hwp', lo, 'hwpfilter/qa/cppunit/data/pass/hangul97-3.0.hwp', '68ff2f51269f3ab619f6465073518cd84694300c', 'MPL-2.0'],
  ['lwp', lo, 'lotuswordpro/qa/cppunit/data/pass/wordpro.lwp', 'dc4048cb92204c15784d78afa2a4b64f2b8c77a4', 'MPL-2.0'],
  ['vor', tika, 'tika-parsers/tika-parsers-standard/tika-parsers-standard-integration-tests/src/test/resources/test-documents/testVORWriterTemplate.vor', '27828eb36f9fdb636000a15b6e262e67a326d2ee', 'Apache-2.0'],
  ['sgl', mime, 'tests/mime-detection/so5.sgl', '5a5bcb7526a1879328555774e27fba5348155f83', 'GPL-2.0 (upstream COPYING)'],
];

await mkdir(root, { recursive: true });
const provenance = [];
// libwps publishes source-authored nonempty DOS fixtures with an explicit COPYING.
const wpsRoot=resolve('.cache/corpora/libwps-reference');
const wpsRevision='bc9019bc453e173112f0b4ac1dee5bbb3aa4ecda';
const wpsPath='Works-2.00A-DOS/LANDSCAP.WPS';
const wpsData=execFileSync('git',['-C',wpsRoot,'show',`${wpsRevision}:${wpsPath}`]);
const wpsBlob=createHash('sha1').update(`blob ${wpsData.length}\0`).update(wpsData).digest('hex');
if(wpsBlob!=='ca1fd9152864407cdb68b8b9254e6fdb1051b737')throw new Error('Works reference mismatch');
await writeFile(resolve(root,'legacy.wps'),wpsData);
provenance.push({file:'legacy.wps',url:`https://sourceforge.net/p/libwps/libwps-reference/ci/${wpsRevision}/tree/${wpsPath}`,gitBlob:wpsBlob,sha256:createHash('sha256').update(wpsData).digest('hex'),license:'GPL-2.0 (upstream COPYING)'});
await mkdir(resolve(root,'negative'),{recursive:true});
await writeFile(resolve(root,'negative/works-blank.wps'),execFileSync('git',['-C',wpsRoot,'show',`${wpsRevision}:Works-2.00A-DOS/BLANK.WPS`]));
for (const [extension, base, path, expected, license] of files) {
  const response = await fetch(base + path, { signal: AbortSignal.timeout(30000) });
  if (!response.ok) throw new Error(`download failed for ${extension}: ${response.status}`);
  const data = Buffer.from(await response.arrayBuffer());
  if (data.length > 32 * 1024 * 1024) throw new Error(`oversized fixture ${extension}`);
  const gitBlob = createHash('sha1').update(`blob ${data.length}\0`).update(data).digest('hex');
  if (gitBlob !== expected) throw new Error(`upstream blob mismatch for ${extension}: ${gitBlob}`);
  const file = `legacy.${extension}`;
  await writeFile(resolve(root, file), data);
  provenance.push({ file, url: base + path, gitBlob, sha256: createHash('sha256').update(data).digest('hex'), license });
  console.log(`verified ${file} (${data.length} bytes)`);
}
await mkdir(resolve(root, 'licenses'), { recursive: true });
for(const name of ['COPYING','README'])await writeFile(resolve(root,'licenses','libwps-'+name+'.txt'),execFileSync('git',['-C',wpsRoot,'show',`${wpsRevision}:Works-2.00A-DOS/${name}`]));
for (const [name, url] of [['MPL-2.0.txt', lo + 'COPYING.MPL'], ['Apache-2.0.txt', tika + 'LICENSE.txt'], ['Tika-NOTICE.txt', tika + 'NOTICE.txt'], ['shared-mime-info-COPYING.txt', mime + 'COPYING']]) {
  const response = await fetch(url, { signal: AbortSignal.timeout(30000) });
  if (!response.ok) throw new Error(`license download failed: ${name}`);
  await writeFile(resolve(root, 'licenses', name), Buffer.from(await response.arrayBuffer()));
}
await writeFile(resolve(root, 'upstream.json'), JSON.stringify(provenance, null, 2) + '\n');
