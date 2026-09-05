(() => {
  const list = document.getElementById('list');
  const feedback = document.getElementById('feedback');
  let generation = 0, nextOutcome = 'success', selectedId = null, pending = false;

  function renderList() {
    list.replaceChildren();
    for (const file of PreviewDemo.list()) {
      const button = document.createElement('button');
      button.className = 'file';
      button.dataset.id = file.id;
      const badge = document.createElement('span');
      badge.className = 'badge ' + (file.id === 'photo' ? 'image' : file.id === 'pdf' ? 'pdf' : '');
      badge.textContent = file.extension;
      const copy = document.createElement('span');
      copy.className = 'file-copy';
      const name = document.createElement('strong');
      name.textContent = file.filename;
      const subtitle = document.createElement('small');
      subtitle.textContent = file.subtitle;
      copy.append(name, subtitle);
      const arrow = document.createElement('span');
      arrow.className = 'chevron'; arrow.textContent = '›'; arrow.setAttribute('aria-hidden', 'true');
      button.append(badge, copy, arrow);
      button.addEventListener('click', () => acquire(file.id));
      list.append(button);
    }
  }

  function show(state) {
    generation++;
    pending = state === 'acquiring';
    list.hidden = state !== 'list';
    document.getElementById('list-heading').hidden = !['list', 'loading'].includes(state);
    document.getElementById('list-note').hidden = state !== 'list';
    feedback.hidden = state === 'list';
    list.setAttribute('aria-busy', String(pending));
    document.querySelectorAll('[data-state]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.state === state)));
    if (state === 'list') { renderList(); return; }
    if (state === 'loading') {
      feedback.innerHTML = '<span class="spinner" aria-hidden="true"></span><h2>正在加载附件</h2><p>请稍候</p><button class="secondary" id="restore">重新加载</button>';
    } else {
      const states = {
        empty: ['—', '暂无示例附件', '附件准备好后将在这里显示。'],
        error: ['!', '附件加载失败', '请检查网络，稍后重新加载。'],
        acquiring: ['loading', '正在获取预览', '正在读取最新文件信息，请稍候。'],
        failed: ['!', '暂时无法获取预览', '请检查网络后重试，或返回选择其他附件。']
      };
      const [symbol, title, description] = states[state];
      const actions = state === 'acquiring' ? '<button class="secondary" id="cancel">取消并返回</button>'
        : state === 'failed' ? '<button class="primary" id="retry">重新获取预览</button><button class="secondary" id="cancel">返回附件列表</button>'
        : '<button class="primary" id="restore">重新加载</button>';
      feedback.innerHTML = '<div class="symbol">' + (symbol === 'loading' ? '<span class="spinner" aria-hidden="true"></span>' : symbol) + '</div><h2>' + title + '</h2><p>' + description + '</p>' + actions;
    }
    document.getElementById('cancel')?.addEventListener('click', () => {
      show('list'); list.querySelector('[data-id="' + selectedId + '"]')?.focus();
    });
    document.getElementById('retry')?.addEventListener('click', () => acquire(selectedId));
    document.getElementById('restore')?.addEventListener('click', () => {
      show('loading'); const current = generation;
      window.setTimeout(() => { if (current === generation) show('list'); }, 450);
    });
  }

  async function acquire(id) {
    if (pending) return;
    selectedId = id;
    const outcome = nextOutcome;
    nextOutcome = 'success';
    document.querySelectorAll('[data-outcome]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.outcome === nextOutcome)));
    show('acquiring'); const current = generation;
    try {
      const detail = await PreviewDemo.detail(id, outcome);
      if (current !== generation) return;
      // Only the mock detail response supplies the next page's synthetic sample ID.
      location.assign('../preview-h5/index.html#sample=' + encodeURIComponent(detail.id));
    } catch {
      if (current === generation) show('failed');
    }
  }

  document.querySelectorAll('[data-state]').forEach(button => button.addEventListener('click', () => show(button.dataset.state)));
  document.querySelectorAll('[data-outcome]').forEach(button => button.addEventListener('click', () => {
    nextOutcome = button.dataset.outcome;
    document.querySelectorAll('[data-outcome]').forEach(item => item.setAttribute('aria-pressed', String(item === button)));
  }));
  window.addEventListener('pagehide', () => { generation++; pending = false; selectedId = null; });
  window.addEventListener('pageshow', event => { if (event.persisted) show('list'); });
  show('list');
})();
