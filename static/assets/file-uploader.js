class FileUploader extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this.shadowRoot.innerHTML = `
      <input type="file" />
      <button id="upload-btn">Upload</button>
      <button id="cancel-btn" disabled>Cancel</button>
      <progress value="0" max="100" style="width:100%;"></progress>
      <div class="status"></div>
    `;
    this.xhr = null;
  }

  connectedCallback() {
    this.uploadUrl = this.getAttribute('upload-url');
    this.useType = this.getAttribute('use-type');

    this.input = this.shadowRoot.querySelector('input[type=file]');
    this.uploadBtn = this.shadowRoot.querySelector('#upload-btn');
    this.cancelBtn = this.shadowRoot.querySelector('#cancel-btn');
    this.progress = this.shadowRoot.querySelector('progress');
    this.status = this.shadowRoot.querySelector('.status');

    this.uploadBtn.addEventListener('click', () => this.uploadFile());
    this.cancelBtn.addEventListener('click', () => this.cancelUpload());
  }

  uploadFile() {
    const file = this.input.files[0];
    if (!file) {
      this.status.textContent = '파일을 선택하세요.';
      return;
    }

    const formData = new FormData();
    formData.append('file', file);
    formData.append('useType', this.useType);

    this.xhr = new XMLHttpRequest();
    this.xhr.open('POST', this.uploadUrl, true);
    this.xhr.withCredentials = true;

    this.xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        this.progress.value = (e.loaded / e.total) * 100;
      }
    };

    this.xhr.onload = () => {
      if (this.xhr.status === 200) {
        this.status.innerHTML = `<span style="color:green">업로드 성공</span>`;
      } else {
        this.status.innerHTML = `<span style="color:red">업로드 실패</span>`;
      }
      this.reset();
    };

    this.xhr.onerror = () => {
      this.status.innerHTML = `<span style="color:red">업로드 중 오류 발생</span>`;
      this.reset();
    };

    this.cancelBtn.disabled = false;
    this.xhr.send(formData);
  }

  cancelUpload() {
    if (this.xhr) {
      this.xhr.abort();
      this.status.innerHTML = `<span style="color:red">업로드 취소됨</span>`;
      this.reset();
    }
  }

  reset() {
    this.progress.value = 0;
    this.cancelBtn.disabled = true;
    this.xhr = null;
  }
}

customElements.define('file-uploader', FileUploader);
