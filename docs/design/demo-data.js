/* Synthetic design fixtures. This file contains no service tokens or source URLs. */
window.PreviewDemo = (() => {
  const files = Object.freeze({
    photo: Object.freeze({ id: 'photo', filename: '山野风景.png', type: 'image', extension: 'PNG', subtitle: '图片 · 查看风景示例' }),
    pdf: Object.freeze({ id: 'pdf', filename: '项目说明.pdf', type: 'pdf', extension: 'PDF', subtitle: 'PDF 文档 · 3 页示例' }),
    office: Object.freeze({ id: 'office', filename: '使用指南.docx', type: 'pdf', extension: 'DOCX', subtitle: 'Office 文档 · 转换后阅读' })
  });
  const exists = id => Object.prototype.hasOwnProperty.call(files, id);
  return Object.freeze({
    list: () => Object.values(files).map(({ id, filename, extension, subtitle }) => ({ id, filename, extension, subtitle })),
    find: id => exists(id) ? files[id] : null,
    detail: (id, outcome = 'success') => new Promise((resolve, reject) => {
      window.setTimeout(() => {
        if (!exists(id) || outcome === 'error') reject(new Error('示例预览获取失败'));
        else resolve({ ...files[id] });
      }, 650);
    })
  });
})();
