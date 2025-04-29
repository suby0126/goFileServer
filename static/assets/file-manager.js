class FileManager extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this.shadowRoot.innerHTML = `
      <table>
        <thead>
          <tr><th>ID</th><th>파일명</th><th>용도</th><th>크기</th><th>업로드일</th><th>액션</th></tr>
        </thead>
        <tbody></tbody>
      </table>
      <div>
        <button id="prev">이전</button>
        <button id="next">다음</button>
      </div>
      <div class="status"></div>
    `;
    this.page = 1;
    this.pageSize = 10;
  }

  connectedCallback() {
    this.tbody = this.shadowRoot.querySelector('tbody');
    this.status = this.shadowRoot.querySelector('.status');
    this.prevBtn = this.shadowRoot.getElementById('prev');
    this.nextBtn = this.shadowRoot.getElementById('next');

    this.listUrl = this.getAttribute('list-url');
    this.downloadUrl = this.getAttribute('download-url');
    this.deleteUrl = this.getAttribute('delete-url');

    this.prevBtn.addEventListener('click', () => { this.page--; this.loadFiles(); });
    this.nextBtn.addEventListener('click', () => { this.page++; this.loadFiles(); });

    this.loadFiles();
  }

  async loadFiles() {
    try {
      const res = await AuthManager.apiFetch(`${this.listUrl}?page=${this.page}&pageSize=${this.pageSize}`);
      const data = await res.json();
      this.renderFiles(data.files);
    } catch {
      this.status.innerHTML = `<span style="color:red">파일 불러오기 실패</span>`;
    }
  }

  renderFiles(files) {
    this.tbody.innerHTML = '';
    files.forEach(f => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${f.id}</td>
        <td>${f.original_name}</td>
        <td>${f.use_type}</td>
        <td>${(f.size / 1024).toFixed(1)} KB</td>
        <td>${new Date(f.created_at).toLocaleString()}</td>
        <td>
          <button data-id="${f.id}" class="download">다운로드</button>
          <button data-id="${f.id}" class="delete">삭제</button>
        </td>
      `;
      this.tbody.appendChild(tr);
    });

    this.shadowRoot.querySelectorAll('.download').forEach(btn => {
      btn.addEventListener('click', (e) => window.open(`${this.downloadUrl}?id=${e.target.dataset.id}`, '_blank'));
    });

    this.shadowRoot.querySelectorAll('.delete').forEach(btn => {
      btn.addEventListener('click', (e) => this.deleteFile(e.target.dataset.id));
    });
  }

  async deleteFile(id) {
    if (!confirm('정말 삭제할까요?')) return;

    const res = await AuthManager.apiFetch(`${this.deleteUrl}?id=${id}`, { method: 'DELETE' });
    if (res.ok) {
      this.loadFiles();
    } else {
      alert('삭제 실패');
    }
  }
}

customElements.define('file-manager', FileManager);
