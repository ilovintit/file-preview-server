(() => {
  const params = new URLSearchParams(location.hash.slice(1));
  const initialFile = PreviewDemo.find(params.get('sample'));
  const requestedState = params.get('state');
  if (location.hash) history.replaceState(null, '', location.pathname + location.search);

  const content = document.getElementById('document-content');
  const viewport = document.getElementById('viewport');
  const feedback = document.getElementById('feedback');
  const toolbar = document.getElementById('toolbar');
  let selected = initialFile, generation = 0, page = 1, zoom = 100;

  const papers = [
    '<p class="eyebrow">FILE PREVIEW / GUIDE</p><h2>每一份文件，<br>都有清晰的打开方式。</h2><img class="illustration" src="../landscape.svg" alt="远山与湖泊的合成插画"><p>从附件列表出发，在阅读页查看图片与文档。这份合成样稿展示文件预览的阅读层级与排版。</p><div class="page-footer">文件预览使用说明 · 01</div>',
    '<p class="eyebrow">READING EXPERIENCE</p><h2>从选择，到阅读</h2><h3>01　选择一份附件</h3><p>在附件列表中找到所需文件，点击后等待获取预览信息。</p><h3>02　查看文件内容</h3><p>图片可以调整显示大小；文档可以逐页阅读。Office 文件准备完成后以 PDF 的阅读方式呈现。</p><h3>03　回到附件列表</h3><p>阅读结束后使用页面顶部的返回入口，继续选择其他文件。</p><div class="page-footer">文件预览使用说明 · 02</div>',
    '<p class="eyebrow">WHEN SOMETHING GOES WRONG</p><h2>遇到问题，也有下一步。</h2><table class="paper-table"><tbody><tr><td>链接失效</td><td>返回附件列表重新获取</td></tr><tr><td>暂时不可用</td><td>稍后重试当前预览</td></tr><tr><td>文件不可预览</td><td>返回选择其他附件</td></tr></tbody></table><p>页面会提供与当前状态对应的操作。加载期间也可以返回，不必一直等待。</p><p>以上内容仅为原型中的合成文档样稿。</p><div class="page-footer">文件预览使用说明 · 03</div>'
  ];

  function syncToolbar() {
    const isImage = selected?.type === 'image';
    for (const id of ['prev', 'next', 'page-counter', 'page-separator']) document.getElementById(id).hidden = isImage;
    document.getElementById('prev').disabled = page === 1;
    document.getElementById('next').disabled = page === papers.length;
    document.getElementById('page-counter').textContent = page + ' / ' + papers.length;
    document.getElementById('zoom-value').textContent = zoom + '%';
    document.getElementById('zoom-out').disabled = zoom === 75;
    document.getElementById('zoom-in').disabled = zoom === 175;
    content.style.zoom = String(zoom / 100);
  }

  function renderDocument() {
    if (!selected) return;
    content.innerHTML = selected.type === 'image'
      ? '<div class="image-canvas"><img src="../landscape.svg" alt="山野风景：远山、日光和湖泊"></div>'
      : '<article class="paper" aria-label="文档第 ' + page + ' 页">' + papers[page - 1] + '</article>';
    syncToolbar();
  }

  function show(state) {
    generation++;
    if (state === 'image') selected = PreviewDemo.find('photo');
    if (state === 'pdf') selected = PreviewDemo.find('pdf');
    if (state === 'office') { selected = PreviewDemo.find('office'); state = 'waiting'; }
    const ready = state === 'ready' || state === 'image' || state === 'pdf';
    document.querySelectorAll('[data-state]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.state === state)));
    document.getElementById('filename').textContent = selected?.filename || '文件预览';
    const kind = document.getElementById('file-kind');
    kind.hidden = !ready; kind.textContent = selected?.type === 'image' ? '图片' : 'PDF';
    toolbar.hidden = !ready; viewport.hidden = !ready; feedback.hidden = ready;
    if (ready) {
      page = 1; zoom = 100; renderDocument();
      document.getElementById('reader-footer').textContent = '合成文件样稿 · 当前仅供查看';
      return;
    }
    content.replaceChildren();
    const states = {
      loading: ['loading', '正在加载文件', '请稍候，即将呈现文件内容。'],
      waiting: ['loading', '正在准备预览', '文件正在转换，请稍候。'],
      expired: ['!', '预览链接已失效', '请返回附件列表，重新获取预览。'],
      revoked: ['!', '预览链接已失效', '请返回附件列表，重新获取预览。'],
      invalid: ['!', '无法预览此文件', '文件内容或格式不符合要求，请返回选择其他附件。'],
      error: ['!', '预览暂不可用', '服务暂时无法处理请求，请稍后重试。'],
      network: ['!', '文件加载失败', '请检查网络后重试，持续失败请联系维护人员。']
    };
    const [symbol, title, description] = states[state] || states.expired;
    const retryable = ['error', 'network'].includes(state) && selected;
    feedback.innerHTML = '<div class="symbol ' + (symbol === '!' ? 'warning' : '') + '">' + (symbol === 'loading' ? '<span class="spinner" aria-hidden="true"></span>' : symbol) + '</div><h2>' + title + '</h2><p>' + description + '</p>'
      + (retryable ? '<button id="retry" class="primary">重试预览</button>' : '')
      + '<a class="secondary" href="../wechat-miniprogram/index.html">返回附件列表</a>';
    document.getElementById('reader-footer').textContent = '预览内容仅供当前查看，请勿分享页面链接。';
    document.getElementById('retry')?.addEventListener('click', () => {
      show('loading'); const current = generation;
      window.setTimeout(() => { if (current === generation) show('ready'); }, 650);
    });
  }

  function officeJourney() {
    show('office'); const current = generation;
    window.setTimeout(() => { if (current === generation) show('ready'); }, 1300);
  }

  document.querySelectorAll('[data-state]').forEach(button => button.addEventListener('click', () => {
    if (button.dataset.state === 'office') officeJourney();
    else show(button.dataset.state);
  }));
  document.getElementById('prev').addEventListener('click', () => { if (page > 1) { page--; renderDocument(); viewport.scrollTop = 0; } });
  document.getElementById('next').addEventListener('click', () => { if (page < papers.length) { page++; renderDocument(); viewport.scrollTop = 0; } });
  document.getElementById('zoom-in').addEventListener('click', () => { zoom = Math.min(175, zoom + 25); syncToolbar(); });
  document.getElementById('zoom-out').addEventListener('click', () => { zoom = Math.max(75, zoom - 25); syncToolbar(); });
  document.getElementById('fit').addEventListener('click', () => { zoom = 100; syncToolbar(); viewport.scrollTop = 0; viewport.scrollLeft = 0; });
  for (const mode of ['wide', 'narrow']) document.getElementById(mode).addEventListener('click', () => {
    document.getElementById('reader').classList.toggle('narrow', mode === 'narrow');
    for (const id of ['wide', 'narrow']) document.getElementById(id).setAttribute('aria-pressed', String(id === mode));
    zoom = 100; syncToolbar();
  });
  window.addEventListener('pagehide', () => { generation++; selected = null; content.replaceChildren(); });
  window.addEventListener('pageshow', event => { if (event.persisted) show('expired'); });

  const allowedStates = ['expired', 'revoked', 'invalid', 'error', 'network', 'loading', 'waiting'];
  if (!initialFile) show('expired');
  else if (allowedStates.includes(requestedState)) show(requestedState);
  else if (initialFile.id === 'office') officeJourney();
  else show('ready');
})();
