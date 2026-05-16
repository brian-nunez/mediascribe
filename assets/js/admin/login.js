    const usernameEl = document.getElementById('username');
    const passwordEl = document.getElementById('password');
    const loginBtnEl = document.getElementById('login_btn');
    const statusEl = document.getElementById('status');

    async function login() {
      statusEl.textContent = 'Logging in...';
      try {
        await AdminCommon.api('/api/admin/login', {
          method: 'POST',
          body: JSON.stringify({ username: usernameEl.value, password: passwordEl.value }),
        });
        window.location.href = '/admin';
      } catch (err) {
        statusEl.textContent = `Login failed: ${err.message}`;
      }
    }

    loginBtnEl.onclick = login;
    passwordEl.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') login();
    });
