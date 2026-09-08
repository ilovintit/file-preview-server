// Fixture-authoring expectations. Never run this to repair a failing CI baseline.
import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const root = resolve('internal/module/preview/testdata/text');
const extensions = 'docm dot dotm dotx odt fodt ott rtf txt wps wpd pages abw zabw lwp mw mcw hwp sxw stw sgl vor 602 bib xml cwk psw uof'.split(' ');
const provenance = JSON.parse(await readFile(resolve(root, 'upstream.json'), 'utf8'));
const common = 'File preview S05 Text 42';
const expected = {
  wps: { pages: 1, text: ["Here's a letter footnote", "Here's a bookmark name."] },
  wpd: { pages: 1, text: ['AND FURTHER', 'test1-2'] },
  pages: { pages: 1, text: ['Document Liberation link.'], exactText: 'Document Liberation link.' },
  lwp: { pages: 2, text: ['This document is created using Lotus Wordpro release 9.6 for Windows.', 'This is a pagebreak.', 'New Page'] },
  mw: { pages: 2, text: ['Classe de première S', 'Exercice 1', 'Exercice 3'] },
  mcw: { pages: 3, text: ['the header', 'the footer', 'line 1', 'yellow'] },
  hwp: { pages: 1, text: ['Hello OpenOffice.org!', '안녕하세요 오픈오피스!'], exactText: 'Hello OpenOffice.org! 안녕하세요 오픈오피스!' },
  sgl: { pages: 1, text: [common], exactText: common },
  vor: { pages: 1, text: ['This is an example StarOffice 5.2 document.'], exactText: 'This is an example StarOffice 5.2 document.' },
  cwk: { pages: 1, text: ['normal', 'vertical center', 'vertical bottom', 'pattern'] },
  psw: { pages: 1, text: ['file format commons psw Pocket Word', '01100110011010010110110001100101'] },
};
const manifest = [];
for (const extension of extensions) {
  const upstream = provenance.find(item => item.file === `legacy.${extension}`);
  const file = upstream ? upstream.file : `source.${extension}`;
  const bytes = await readFile(resolve(root, file));
  const sha256 = createHash('sha256').update(bytes).digest('hex');
  if (upstream && upstream.sha256 !== sha256) throw new Error(`provenance hash mismatch: ${file}`);
  let expectation = expected[extension];
  if (!expectation) {
    const exactText = extension === '602' ? common : extension === 'bib' ? bytes.toString('utf8') : `${common} Résumé 中文`;
    expectation = { pages: 1, text: [common], exactText };
  }
  manifest.push({ extension, file, sha256, ...expectation, source: upstream || { kind: 'repository-authored', source: 'source.fodt', generators: ['generate-text-fixtures.sh', 'generate-text-containers.go'] } });
}
await writeFile(resolve(root, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
console.log(`Froze ${manifest.length} input hashes and source-based expectations; no CI output used.`);
