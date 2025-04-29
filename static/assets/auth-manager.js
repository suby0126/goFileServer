class AuthManager extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this.shadowRoot.innerHTML = `
      <div class="auth">
        <div class="login-form">
          <input type="text" placeholder="Username" id="username">
          <input type="password" placeholder="Password" id="password">
          <button id="login-btn">Login</button>
        </div>
        <div class="logout-form" style="display:none;">
          <span>Logged in</span>
          <button id="logout-btn">Logout</button>
        </div>
        <div class="status"></div>
      </div>
    `;
  }

  connectedCallback() {
    this.loginBtn = this.shadowRoot.querySelector('#login-btn');
    this.logoutBtn = this.shadowRoot.querySelector('#logout-btn');
    this.status = this.shadowRoot.querySelector('.status');

    this.loginBtn.addEventListener('click', () => this.login());
    this.logoutBtn.addEventListener('click', () => this.logout());

    if (AuthManager.isLoggedIn()) {
      this.showLoggedIn();
    }
  }

  async login() {
    const username = this.shadowRoot.getElementById('username').value;
    const password = this.shadowRoot.getElementById('password').value;

    try {
      const res = await fetch('/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({ username, password }),
        credentials: 'include'
      });
      const data = await res.json();
      if (res.ok) {
        this.showLoggedIn();
      } else {
        this.status.innerHTML = `<span style="color:red">${data.error}</span>`;
      }
    } catch (err) {
      this.status.innerHTML = `<span style="color:red">Login failed</span>`;
    }
  }

  logout() {
    document.cookie = 'access_token=; Max-Age=0; path=/';
    document.cookie = 'refresh_token=; Max-Age=0; path=/';
    location.reload();
  }

  showLoggedIn() {
    this.shadowRoot.querySelector('.login-form').style.display = 'none';
    this.shadowRoot.querySelector('.logout-form').style.display = 'block';
  }

  static isLoggedIn() {
    return document.cookie.includes('access_token');
  }

  static async apiFetch(url, options = {}, retry = true) {
    if (!options.credentials) options.credentials = 'include';

    let res = await fetch(url, options);

    if (res.status === 401 && retry) {
      await AuthManager.refreshToken();
      return AuthManager.apiFetch(url, options, false);
    }

    return res;
  }

  static async refreshToken() {
    const res = await fetch('/refresh', {
      method: 'POST',
      credentials: 'include'
    });
    return res.ok;
  }
}

customElements.define('auth-manager', AuthManager);
