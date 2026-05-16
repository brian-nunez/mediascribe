    marked.setOptions({ gfm: true, breaks: true });

    const contentEl = document.getElementById('content');

    function escapeHTML(value) {
      return String(value || '')
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#039;');
    }

    function findBlogID() {
      const pathname = window.location.pathname || '';
      if (pathname.startsWith('/blog/')) return decodeURIComponent(pathname.slice('/blog/'.length));
      const q = new URLSearchParams(window.location.search);
      return q.get('id') || '';
    }

    function dateLabel(value) {
      if (!value) return '';
      const d = new Date(value);
      if (Number.isNaN(d.getTime())) return '';
      return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    }

    function sourceHTML(blog) {
      const source = blog.source_url || blog.source_path || '';
      if (!source) return '<span class="text-slate-500">unavailable</span>';
      if (/^https?:\/\//i.test(source)) {
        return `<a href="${escapeHTML(source)}" target="_blank" rel="noopener noreferrer" class="break-all text-slate-800 underline decoration-slate-300 underline-offset-2 hover:text-black">${escapeHTML(source)}</a>`;
      }
      return `<span class="break-all font-mono text-[11px] text-slate-700">${escapeHTML(source)}</span>`;
    }

    function languageList(blog) {
      return Array.isArray(blog.languages) ? blog.languages : [];
    }

    function readInitialBlog() {
      const el = document.getElementById('initial-blog-data');
      if (!el || !el.textContent) return null;
      try { return JSON.parse(el.textContent); }
      catch (_) { return null; }
    }

    async function loadBlog(id) {
      const res = await fetch(`/api/public/blogs/${encodeURIComponent(id)}`);
      if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
      const payload = await res.json();
      return payload.blog || null;
    }

    function renderBlog(blog) {
      const languages = languageList(blog);
      let selectedLanguage = new URLSearchParams(window.location.search).get('lang') || 'en';
      if (!languages.find(x => x.language === selectedLanguage)) {
        selectedLanguage = languages.find(x => x.language === 'en')?.language || (languages[0]?.language || 'en');
      }

      const q = new URLSearchParams(window.location.search);
      const chunk = q.get('chunk') || '';
      const query = q.get('q') || '';

      const renderLanguage = () => {
        const selected = languages.find(x => x.language === selectedLanguage) || languages[0] || { markdown: '' };
        const markdownRaw = String(selected.markdown || '');
        const tabs = languages.map((item) => {
          const active = item.language === selectedLanguage;
          return `<button data-lang="${escapeHTML(item.language)}" class="rounded-full border px-2.5 py-1 text-xs transition ${active ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-300 bg-white text-slate-600 hover:bg-slate-100'}">${escapeHTML(item.language)}</button>`;
        }).join('');

        const matchHTML = chunk
          ? `<p class="mt-3 rounded-md border border-slate-300 bg-slate-100 px-2 py-1.5 text-xs text-slate-700">Opened from search match: chunk ${escapeHTML(chunk)}${query ? ` for "${escapeHTML(query)}"` : ''}</p>`
          : '';

        contentEl.innerHTML = `
          <span class="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--primary)]" data-blog-section>${escapeHTML(blog.section_name || 'Unsectioned')}</span>
          <h1 class="headline mt-2 text-[2.45rem] leading-[1.08] text-slate-900" data-blog-title>${escapeHTML(blog.title || 'Untitled')}</h1>
          <div class="mt-3 flex flex-wrap items-center gap-2 text-sm text-slate-500" data-blog-meta>
            <span>Job ${(blog.job_id || '').slice(0, 8)}</span>
            <span>•</span>
            <span>${escapeHTML(dateLabel(blog.updated_at))}</span>
          </div>
          ${matchHTML}
          <div class="mt-4 border-y border-[var(--line)] py-3">
            <div class="flex flex-wrap gap-1.5" id="lang_tabs">${tabs}</div>
          </div>
          <div class="mt-4 rounded-lg border border-[var(--line)] bg-slate-50 px-3 py-2">
            <p class="text-[10px] font-semibold uppercase tracking-wide text-slate-500">Video Source</p>
            <div class="mt-1 text-xs">${sourceHTML(blog)}</div>
          </div>
          <div id="article" class="mt-5">${marked.parse(markdownRaw)}</div>
          <details class="mt-6 rounded-lg border border-[var(--line)] bg-slate-50 p-3">
            <summary class="cursor-pointer text-xs font-semibold uppercase tracking-wide text-slate-500">Transcript</summary>
            <pre class="mt-2 max-h-72 overflow-auto whitespace-pre-wrap rounded-lg border border-slate-200 bg-white p-3 text-xs leading-relaxed text-slate-700">${escapeHTML(blog.transcript || 'Transcript unavailable')}</pre>
          </details>
        `;

        const articleEl = document.getElementById('article');
        if (articleEl && typeof renderMathInElement === 'function') {
          renderMathInElement(articleEl, {
            throwOnError: false,
            strict: 'ignore',
            delimiters: [
              { left: '$$', right: '$$', display: true },
              { left: '$', right: '$', display: false },
              { left: '\\\\[', right: '\\\\]', display: true },
              { left: '\\\\(', right: '\\\\)', display: false },
            ],
            ignoredTags: ['script', 'noscript', 'style', 'textarea', 'pre', 'code'],
          });
        }
        renderMermaidBlocks(articleEl);

        contentEl.querySelectorAll('[data-lang]').forEach(btn => {
          btn.onclick = () => {
            selectedLanguage = btn.getAttribute('data-lang') || 'en';
            const u = new URL(window.location.href);
            u.searchParams.set('lang', selectedLanguage);
            history.replaceState(null, '', u.pathname + u.search);
            renderLanguage();
          };
        });
      };

      renderLanguage();
    }

    async function renderMermaidBlocks(articleEl) {
      if (!articleEl) return;
      const blocks = articleEl.querySelectorAll('pre > code.language-mermaid, pre > code.lang-mermaid');
      if (!blocks.length) return;

      const mermaid = window.__mermaid;
      if (!mermaid || typeof mermaid.run !== 'function') return;
      for (let idx = 0; idx < blocks.length; idx += 1) {
        const code = blocks[idx];
        const pre = code.parentElement;
        if (!pre || pre.tagName !== 'PRE') continue;
        const source = (code.textContent || '').trim();
        if (!source) continue;

        const host = document.createElement('div');
        host.className = 'mermaid';
        const renderID = `mermaid-${Date.now()}-${idx}`;

        try {
          const out = await mermaid.render(renderID, source);
          host.innerHTML = out.svg;
          pre.replaceWith(host);
        } catch (_) {
          const fallback = document.createElement('div');
          fallback.className = 'mermaid-fallback';
          fallback.innerHTML = `
            <div class="mermaid-fallback-head">Diagram syntax issue (showing source)</div>
            <pre><code>${escapeHTML(source)}</code></pre>
          `;
          pre.replaceWith(fallback);
        }
      }
    }

    function boot() {
      if ('serviceWorker' in navigator) {
        try { await navigator.serviceWorker.register('/sw.js'); } catch (_) {}
      }

      window.addEventListener('pageswap', (event) => {
        if (!event.viewTransition) return;

        const toUrl = new URL(event.activation.entry.url);
        // If navigating back to the feed, ensure transition names are set
        if (toUrl.pathname === '/' || toUrl.pathname === '/index.html') {
          // Names should already be set via CSS for body[data-page="blog"], 
          // but we can ensure they are explicitly active if needed.
          // In this case, CSS takes care of it because it's the blog page.
        }
      });

      const id = findBlogID();
      if (!id) {
        contentEl.innerHTML = '<p class="text-sm text-red-700">Missing blog id in URL.</p>';
        return;
      }

      try {
        const initial = readInitialBlog();
        const blog = initial && initial.found && initial.blog && initial.blog.id === id
          ? initial.blog
          : await loadBlog(id);
        if (!blog) {
          contentEl.innerHTML = '<p class="text-sm text-red-700">Blog not found.</p>';
          return;
        }
        renderBlog(blog);
      } catch (err) {
        contentEl.innerHTML = `<p class="text-sm text-red-700">Failed to load blog: ${escapeHTML(err.message)}</p>`;
      }
    }

    boot();
