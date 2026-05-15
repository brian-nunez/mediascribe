const AdminCommon = (() => {
  function escapeHTML(value) {
    return String(value || '')
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#039;');
  }

  async function api(path, opts = {}) {
    const res = await fetch(path, {
      headers: { 'content-type': 'application/json' },
      ...opts,
    });
    if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
    return res.json();
  }

  async function requireAuth() {
    try {
      const out = await api('/api/admin/me');
      return out.user;
    } catch (_) {
      window.location.href = '/admin/login';
      return null;
    }
  }

  async function logout() {
    try {
      await api('/api/admin/logout', { method: 'POST' });
    } catch (_) {}
    window.location.href = '/admin/login';
  }

  function sourceLinkHTML(blog) {
    const source = String(blog.source_url || blog.source_path || '').trim();
    if (!source) return '<span class="text-slate-500">No source</span>';
    if (/^https?:\/\//i.test(source)) {
      return `<a href="${escapeHTML(source)}" target="_blank" rel="noopener noreferrer" class="text-blue-700 underline decoration-blue-300 underline-offset-2 hover:text-blue-800 break-all">${escapeHTML(source)}</a>`;
    }
    return `<span class="font-mono text-[11px] text-slate-700 break-all">${escapeHTML(source)}</span>`;
  }

  async function loadAdminCatalog() {
    return api('/api/admin/catalog');
  }

  function navHTML(active) {
    const item = (href, label, key) => {
      const on = active === key;
      return `<a href="${href}" class="rounded-lg px-3 py-2 text-sm font-medium ${on ? 'bg-slate-900 text-white' : 'text-slate-700 hover:bg-slate-100'}">${label}</a>`;
    };
    return `
      <header class="mb-5 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
        <div class="flex items-center justify-between gap-3">
          <div>
            <h1 class="text-xl font-semibold">Admin</h1>
            <p class="text-sm text-slate-500">Clear workflow: sections -> blogs -> edit -> publish.</p>
          </div>
          <div class="flex items-center gap-2">
            <a href="/" class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">Public</a>
            <button id="logout_btn" class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">Logout</button>
          </div>
        </div>
        <nav class="mt-3 flex flex-wrap gap-2">
          ${item('/admin', 'Dashboard', 'dashboard')}
          ${item('/admin/batches', 'Batches', 'batches')}
          ${item('/admin/sections', 'Sections', 'sections')}
          ${item('/admin/blogs', 'Blogs', 'blogs')}
        </nav>
      </header>
    `;
  }

  if (typeof window !== 'undefined' && 'serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('/sw.js').catch(() => {});
    });
  }

  return { escapeHTML, api, requireAuth, logout, sourceLinkHTML, loadAdminCatalog, navHTML };
})();
