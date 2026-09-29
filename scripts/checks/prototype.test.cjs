const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { resolve } = require('node:path');
const { JSDOM } = require('jsdom');

// CI only: no renderer, network fetch, platform SDK or actual service call.
function page(t, entry, fragment = '') {
  const folder = resolve('docs/design', entry);
  const dom = new JSDOM(readFileSync(resolve(folder, 'index.html'), 'utf8'), {
    url: 'https://prototype.invalid/' + entry + '/index.html' + fragment,
    runScripts: 'outside-only'
  });
  t.after(() => dom.window.close());
  const timers = [];
  dom.window.setTimeout = callback => { timers.push(callback); return timers.length; };
  dom.window.eval(readFileSync('docs/design/demo-data.js', 'utf8'));
  dom.window.eval(readFileSync(resolve(folder, 'prototype.js'), 'utf8'));
  const document = dom.window.document;
  return {
    window: dom.window,
    get: selector => document.querySelector(selector),
    click: selector => { const element = document.querySelector(selector); assert.ok(element, selector); element.click(); },
    async tick() { const callback = timers.shift(); assert.ok(callback, 'queued transition'); callback(); await Promise.resolve(); await Promise.resolve(); }
  };
}

test('all three attachment types are discoverable and cancellation defeats a late response', async t => {
  const p = page(t, 'wechat-miniprogram');
  for (const id of ['photo', 'pdf', 'office']) assert.ok(p.get('[data-id="' + id + '"]'));
  p.click('[data-id="office"]');
  assert.match(p.get('#feedback').textContent, /正在获取预览/);
  p.click('#cancel'); await p.tick();
  assert.equal(p.get('#list').hidden, false);
  assert.equal(p.window.location.pathname, '/wechat-miniprogram/index.html');
});

test('detail failure keeps a recovery path and list scenarios can reload', async t => {
  const p = page(t, 'wechat-miniprogram');
  p.click('[data-outcome="error"]'); p.click('[data-id="pdf"]'); await p.tick();
  assert.match(p.get('#feedback').textContent, /暂时无法获取预览/);
  p.click('#retry'); assert.match(p.get('#feedback').textContent, /正在获取预览/);
  p.click('#cancel'); await p.tick();
  for (const state of ['empty', 'error', 'loading']) {
    p.click('[data-state="' + state + '"]');
    assert.equal(p.get('#list').hidden, true);
    p.click('#restore'); await p.tick();
    assert.equal(p.get('#list').hidden, false);
  }
});

test('fragment is cleared; absent or unrecognized sample never opens a document', t => {
  for (const fragment of ['', '#sample=unknown', '#sample=constructor']) {
    const p = page(t, 'preview-h5', fragment);
    assert.equal(p.window.location.hash, '');
    assert.equal(p.get('#viewport').hidden, true);
    assert.match(p.get('#feedback').textContent, /预览链接已失效/);
  }
});

test('PDF pagination and zoom controls enforce first, last and size boundaries', t => {
  const p = page(t, 'preview-h5', '#sample=pdf');
  assert.equal(p.window.location.hash, '');
  assert.equal(p.get('#viewport').hidden, false);
  assert.equal(p.get('#prev').disabled, true);
  p.click('#next'); assert.equal(p.get('#page-counter').textContent, '2 / 3');
  assert.match(p.get('#document-content').textContent, /从选择，到阅读/);
  p.click('#next'); assert.equal(p.get('#next').disabled, true);
  p.click('#prev'); assert.equal(p.get('#page-counter').textContent, '2 / 3');
  for (let i = 0; i < 6; i++) p.click('#zoom-in');
  assert.equal(p.get('#zoom-value').textContent, '175%');
  assert.equal(p.get('#zoom-in').disabled, true);
  p.click('#fit'); assert.equal(p.get('#zoom-value').textContent, '100%');
  p.click('#zoom-out'); assert.equal(p.get('#zoom-out').disabled, true);
});

test('image mode has no pagination; narrow mode changes the presentation shell', t => {
  const p = page(t, 'preview-h5', '#sample=photo');
  assert.equal(p.get('#prev').hidden, true);
  assert.equal(p.get('#file-kind').textContent, '图片');
  assert.ok(p.get('.image-canvas img').getAttribute('alt'));
  p.click('#narrow'); assert.ok(p.get('#reader').classList.contains('narrow'));
  p.click('#wide'); assert.equal(p.get('#reader').classList.contains('narrow'), false);
});

test('Office waits then becomes PDF; a newer expired state wins over conversion', async t => {
  const p = page(t, 'preview-h5', '#sample=office');
  assert.match(p.get('#feedback').textContent, /正在准备预览/);
  await p.tick(); assert.equal(p.get('#file-kind').textContent, 'PDF');
  assert.equal(p.get('#filename').textContent, '使用指南.docx');
  p.click('[data-state="office"]'); p.click('[data-state="expired"]'); await p.tick();
  assert.equal(p.get('#viewport').hidden, true);
  assert.match(p.get('#feedback').textContent, /预览链接已失效/);
});

test('revocation and expiry are indistinguishable; 422 cannot retry, 5xx can', async t => {
  const p = page(t, 'preview-h5', '#sample=pdf');
  p.click('[data-state="expired"]'); const expired = p.get('#feedback').innerHTML;
  p.click('[data-state="revoked"]'); assert.equal(p.get('#feedback').innerHTML, expired);
  p.click('[data-state="invalid"]'); assert.equal(p.get('#retry'), null);
  p.click('[data-state="error"]'); p.click('#retry');
  assert.equal(p.get('#retry'), null); await p.tick();
  assert.equal(p.get('#viewport').hidden, false);
  p.click('[data-state="network"]'); assert.match(p.get('#feedback').textContent, /文件加载失败/);
});

test('leaving the reading page clears content and cached page restoration expires it', t => {
  const p = page(t, 'preview-h5', '#sample=pdf');
  p.window.dispatchEvent(new p.window.PageTransitionEvent('pagehide'));
  assert.equal(p.get('#document-content').textContent, '');
  p.window.dispatchEvent(new p.window.PageTransitionEvent('pageshow', { persisted: true }));
  assert.equal(p.get('#viewport').hidden, true);
  assert.match(p.get('#feedback').textContent, /预览链接已失效/);
});
