    const sectionListEl = document.getElementById('section_list');
    const sectionSearchEl = document.getElementById('section_search');
    const languageListEl = document.getElementById('language_list');
    const feedEl = document.getElementById('feed');
    const feedMetaEl = document.getElementById('feed_meta');
    const searchInputEl = document.getElementById('search_input');
    const searchDropdownEl = document.getElementById('search_dropdown');
    const menuBtnEl = document.getElementById('menu_btn');
    const leftNavEl = document.getElementById('left_nav');
    const mobileBackdropEl = document.getElementById('mobile_backdrop');

    const state = {
      sections: [{ id: 'all', name: 'All', count: 0 }],
      items: [],
      selectedSection: 'all',
      query: '',
      searchResults: [],
      searchMap: new Map(),
      searchTimer: null,
      searchOpen: false,
      offset: 0,
      limit: 20,
      total: 0,
      hasMore: true,
      loading: false,
      sectionFilter: '',
      selectedLanguage: 'all',
      languageCounts: { all: 0 },
      navOpen: false,
    };

    function readInitialFeed() {
      const el = document.getElementById('initial-feed-data');
      if (!el || !el.textContent) return null;
      try { return JSON.parse(el.textContent); }
      catch (_) { return null; }
    }

    function applyNavState() {
      const desktop = window.matchMedia('(min-width: 768px)').matches;
      if (desktop) {
        mobileBackdropEl.classList.add('hidden');
        if (state.navOpen) leftNavEl.classList.add('md:hidden');
        else leftNavEl.classList.remove('md:hidden');
        leftNavEl.classList.remove('-translate-x-full');
        return;
      }
      leftNavEl.classList.remove('md:hidden');
      if (state.navOpen) {
        leftNavEl.classList.remove('-translate-x-full');
        mobileBackdropEl.classList.remove('hidden');
      } else {
        leftNavEl.classList.add('-translate-x-full');
        mobileBackdropEl.classList.add('hidden');
      }
    }

    function escapeHTML(value) {
      return String(value || '')
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#039;');
    }

    function dateLabel(value) {
      const d = new Date(value || 0);
      if (Number.isNaN(d.getTime())) return '';
      return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    }

    function sourceDomain(source) {
      if (!source) return 'no source';
      if (/^https?:\/\//i.test(source)) {
        try { return new URL(source).hostname.replace(/^www\./, ''); }
        catch (_) { return source; }
      }
      return 'local path';
    }

    function blogURL(blog) {
      return `/blog/${encodeURIComponent(blog.id)}?lang=en`;
    }

    function isBlogNavigation(event, link) {
      if (!link || event.defaultPrevented || event.button !== 0) return false;
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return false;
      if (link.target && link.target !== '_self') return false;
      let url;
      try { url = new URL(link.href, window.location.href); }
      catch (_) { return false; }
      return url.origin === window.location.origin && url.pathname.startsWith('/blog/');
    }

    function clearTransitionSource() {
      document.querySelectorAll('[data-vt-active]').forEach((el) => {
        el.style.viewTransitionName = '';
        el.style.contain = '';
        delete el.dataset.vtActive;
      });
    }

    function armBlogTransition(link) {
      if (!window.CSS || !CSS.supports('view-transition-name: blog-title')) return;
      clearTransitionSource();

      const row = link.closest('[data-blog-card]') || link;
      const title = row.querySelector('[data-blog-title]');
      const meta = row.querySelector('[data-blog-meta]');

      row.style.viewTransitionName = 'blog-shell';
      row.style.contain = 'layout';
      row.dataset.vtActive = 'true';

      if (title) {
        title.style.viewTransitionName = 'blog-title';
        title.style.contain = 'layout';
        title.dataset.vtActive = 'true';
      }
      if (meta) {
        meta.style.viewTransitionName = 'blog-meta';
        meta.style.contain = 'layout';
        meta.dataset.vtActive = 'true';
      }
    }

    // Handle cross-document view transitions for the "back" navigation
    function handlePageReveal(event) {
      if (!event.viewTransition) return;

      const navigation = window.navigation?.activation;
      if (!navigation) return;

      const fromUrl = new URL(navigation.from?.url || '');
      const toUrl = new URL(navigation.entry?.url || '');

      // If we are coming back from a blog page to the feed
      if (fromUrl.pathname.startsWith('/blog/') && (toUrl.pathname === '/' || toUrl.pathname === '/index.html')) {
        const blogId = decodeURIComponent(fromUrl.pathname.split('/')[2]);
        if (!blogId) return;

        // Try to find the matching blog card in the feed
        const findAndArm = () => {
          const card = document.querySelector(`[data-blog-id="${blogId}"]`);
          if (card) {
            armBlogTransition(card);
            return true;
          }
          return false;
        };

        if (!findAndArm()) {
          // If not found yet, it might be due to async rendering
          const observer = new MutationObserver(() => {
            if (findAndArm()) observer.disconnect();
          });
          observer.observe(feedEl, { childList: true, subtree: true });
          setTimeout(() => observer.disconnect(), 2000);
        }
      }
    }

    window.addEventListener('pagereveal', handlePageReveal);

    function searchSnippet(content) {
      const text = String(content || '').replace(/\s+/g, ' ').trim();
      if (text.length <= 170) return text;
      return `${text.slice(0, 170)}...`;
    }

    function timeLabel(total) {
      const n = Number(total || 0);
      if (!Number.isFinite(n) || n < 0) return '--:--';
      const sec = Math.floor(n);
      const m = Math.floor(sec / 60);
      const s = sec % 60;
      return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
    }

    function sectionButton(section, active) {
      return `<button data-section="${escapeHTML(section.id)}" class="w-full rounded-full px-3 py-2 text-left text-sm transition ${active ? 'bg-[var(--primary)] text-white' : 'text-slate-700 hover:bg-slate-100'}">${escapeHTML(section.name)} (${Number(section.count || 0)})</button>`;
    }

    function languageButton(lang, count, active) {
      const label = lang === 'all' ? 'All languages' : lang;
      return `<button data-language="${escapeHTML(lang)}" class="w-full rounded-full px-3 py-1.5 text-left text-xs transition ${active ? 'bg-slate-900 text-white' : 'text-slate-700 hover:bg-slate-100'}">${escapeHTML(label)} (${count})</button>`;
    }

    function renderSections() {
      const visible = state.sections.filter(s => s.id === 'all' || s.count > 0);
      const q = state.sectionFilter.trim().toLowerCase();
      const filtered = visible.filter(s => s.id === 'all' || !q || s.name.toLowerCase().includes(q));
      sectionListEl.innerHTML = filtered.map(section => sectionButton(section, section.id === state.selectedSection)).join('');
      sectionListEl.querySelectorAll('[data-section]').forEach(btn => {
        btn.onclick = async () => {
          state.selectedSection = btn.getAttribute('data-section') || 'all';
          renderSections();
          await resetAndLoadFeed();
        };
      });
    }

    function renderLanguageFilter() {
      const counts = new Map();
      const apiCounts = state.languageCounts || {};
      let total = 0;
      for (const [k, v] of Object.entries(apiCounts)) {
        const key = String(k || '').toLowerCase();
        const n = Number(v || 0);
        if (!key || key === 'all' || !Number.isFinite(n)) continue;
        counts.set(key, n);
        total += n;
      }
      counts.set('all', total);
      const langs = Array.from(counts.keys()).filter(k => k !== 'all').sort();
      languageListEl.innerHTML = ['all', ...langs]
        .filter((lang) => lang === 'all' || (counts.get(lang) || 0) > 0)
        .map((lang) => languageButton(lang, counts.get(lang) || 0, lang === state.selectedLanguage))
        .join('');
      languageListEl.querySelectorAll('[data-language]').forEach((btn) => {
        btn.onclick = async () => {
          state.selectedLanguage = btn.getAttribute('data-language') || 'all';
          renderLanguageFilter();
          await resetAndLoadFeed();
        };
      });
    }

    function visibleBlogs() {
      let items = state.items;
      if (state.selectedLanguage !== 'all') {
        items = items.filter((b) => Array.isArray(b.languages) && b.languages.includes(state.selectedLanguage));
      }
      return items;
    }

    function renderFeed() {
      const blogs = visibleBlogs();
      feedEl.innerHTML = '';
      if (!blogs.length) {
        feedMetaEl.textContent = state.loading ? 'Loading…' : 'No blogs for this filter.';
        feedEl.innerHTML = '<div class="py-12 text-sm text-slate-500">Try another section or language filter.</div>';
        return;
      }
      feedMetaEl.textContent = `${blogs.length} of ${state.total} blog${state.total === 1 ? '' : 's'}`;

      for (const blog of blogs) {
        const source = blog.source_url || blog.source_path || '';
        const row = document.createElement('a');
        row.href = blogURL(blog);
        row.className = 'group block py-8';
        row.dataset.blogCard = '';
        row.dataset.blogId = blog.id;
        row.innerHTML = `
          <div class="text-xs text-slate-500" data-blog-meta>${escapeHTML(blog.section_name || 'Unsectioned')} • ${escapeHTML(dateLabel(blog.updated_at))}</div>
          <h2 class="mt-2 brand text-[2rem] leading-tight text-slate-900 transition group-hover:text-[var(--primary)]" data-blog-title>${escapeHTML(blog.title || 'Untitled')}</h2>
          <p class="mt-2 text-lg leading-relaxed text-slate-600" data-blog-preview>${escapeHTML(blog.preview || '')}</p>
          <div class="mt-3 text-xs text-slate-500">${escapeHTML(sourceDomain(source))} • ${(blog.languages || []).length} language${(blog.languages || []).length === 1 ? '' : 's'}</div>
        `;
        feedEl.appendChild(row);
      }

      if (state.loading) {
        const loading = document.createElement('div');
        loading.className = 'py-4 text-xs text-slate-500';
        loading.textContent = 'Loading more…';
        feedEl.appendChild(loading);
      }
    }

    async function loadFeedPage() {
      if (state.loading || !state.hasMore) return;
      state.loading = true;
      renderFeed();

      const params = new URLSearchParams();
      params.set('limit', String(state.limit));
      params.set('offset', String(state.offset));
      if (state.selectedSection !== 'all') params.set('section_id', state.selectedSection);
      if (state.selectedLanguage !== 'all') params.set('lang', state.selectedLanguage);

      const res = await fetch(`/api/public/feed?${params.toString()}`);
      if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
      const page = await res.json();

      state.sections = [{ id: 'all', name: 'All', count: Number(page.total || 0) }].concat(
        (Array.isArray(page.sections) ? page.sections : []).map(s => ({ id: s.id || '', name: s.name || 'Section', count: Number(s.count || 0) }))
      );
      state.items = state.items.concat(Array.isArray(page.items) ? page.items : []);
      state.offset = Number(page.next_offset || state.items.length);
      state.total = Number(page.total || state.total);
      state.languageCounts = page.language_counts || {};
      state.hasMore = Boolean(page.has_more);
      state.loading = false;
      renderSections();
      renderLanguageFilter();
      renderFeed();
    }

    async function resetAndLoadFeed() {
      state.items = [];
      state.offset = 0;
      state.total = 0;
      state.hasMore = true;
      state.loading = false;
      renderFeed();
      await loadFeedPage();
    }

    function hydrateInitialFeed(initial) {
      const page = initial && initial.page ? initial.page : null;
      if (!page) return false;
      state.selectedSection = initial.selected_section || 'all';
      state.selectedLanguage = initial.selected_language || 'all';
      state.items = Array.isArray(page.items) ? page.items : [];
      state.offset = Number(page.next_offset || state.items.length);
      state.total = Number(page.total || state.items.length);
      state.limit = Number(page.limit || state.limit);
      state.hasMore = Boolean(page.has_more);
      state.languageCounts = page.language_counts || {};
      state.sections = [{ id: 'all', name: 'All', count: Number(page.total || 0) }].concat(
        (Array.isArray(page.sections) ? page.sections : []).map(s => ({ id: s.id || '', name: s.name || 'Section', count: Number(s.count || 0) }))
      );
      renderSections();
      renderLanguageFilter();
      renderFeed();
      return true;
    }

    function renderSearchDropdown() {
      searchDropdownEl.innerHTML = '';
      if (!state.searchOpen || !state.query.trim()) {
        searchDropdownEl.classList.add('hidden');
        return;
      }

      if (!state.searchResults.length) {
        searchDropdownEl.classList.remove('hidden');
        searchDropdownEl.innerHTML = '<p class="px-2 py-3 text-sm text-slate-500">No relevant matches found.</p>';
        return;
      }

      searchDropdownEl.classList.remove('hidden');
      for (const item of state.searchResults) {
        const row = document.createElement('a');
        row.href = `/blog/${encodeURIComponent(item.blog_id)}?lang=en&q=${encodeURIComponent(state.query.trim())}`;
        row.className = 'block rounded-lg px-2 py-2 text-sm hover:bg-slate-100';
        row.dataset.blogCard = '';
        row.innerHTML = `
          <div class="flex items-center justify-between gap-2">
            <p class="font-semibold text-slate-800" data-blog-title>${escapeHTML(item.title || 'Untitled')}</p>
            <span class="text-[10px] uppercase tracking-wide text-slate-500" data-blog-meta>${escapeHTML(item.section_name || 'Unsectioned')}</span>
          </div>
          <p class="mt-1 text-xs text-slate-600">${escapeHTML(searchSnippet(item.preview || ''))}</p>
          <p class="mt-1 text-[11px] text-slate-500">Best match at ${timeLabel(item.match_start_seconds)}-${timeLabel(item.match_end_seconds)}</p>
        `;
        searchDropdownEl.appendChild(row);
      }
    }

    async function runSearch(query) {
      const q = String(query || '').trim();
      if (!q) {
        state.searchResults = [];
        state.searchMap.clear();
        renderSearchDropdown();
        return;
      }
      const res = await fetch(`/api/search?q=${encodeURIComponent(q)}&limit=20`);
      if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
      const payload = await res.json();
      const raw = Array.isArray(payload.results) ? payload.results : [];
      state.searchResults = raw;
      renderSearchDropdown();
    }

    async function boot() {
      if ('serviceWorker' in navigator) {
        try { await navigator.serviceWorker.register('/sw.js'); } catch (_) {}
      }
      if (!hydrateInitialFeed(readInitialFeed())) {
        await resetAndLoadFeed();
      }
      applyNavState();

      const sentinel = document.createElement('div');
      sentinel.className = 'h-8';
      feedEl.parentElement.appendChild(sentinel);
      const io = new IntersectionObserver(async (entries) => {
        for (const e of entries) {
          if (e.isIntersecting && state.hasMore && !state.loading) {
            try { await loadFeedPage(); } catch (_) {}
          }
        }
      }, { rootMargin: '300px 0px 300px 0px' });
      io.observe(sentinel);

      searchInputEl.addEventListener('focus', () => {
        state.searchOpen = true;
        renderSearchDropdown();
      });
      searchInputEl.addEventListener('blur', () => {
        setTimeout(() => {
          state.searchOpen = false;
          renderSearchDropdown();
        }, 120);
      });
      searchInputEl.addEventListener('input', () => {
        state.query = searchInputEl.value;
        if (state.searchTimer) clearTimeout(state.searchTimer);
        state.searchTimer = setTimeout(async () => {
          try {
            await runSearch(state.query);
          } catch (_) {
            state.searchResults = [];
            renderSearchDropdown();
          }
        }, 250);
      });

      sectionSearchEl.addEventListener('input', () => {
        state.sectionFilter = sectionSearchEl.value || '';
        renderSections();
      });

      menuBtnEl.addEventListener('click', () => {
        state.navOpen = !state.navOpen;
        applyNavState();
      });
      mobileBackdropEl.addEventListener('click', () => {
        state.navOpen = false;
        applyNavState();
      });
      document.addEventListener('click', (event) => {
        const target = event.target instanceof Element ? event.target : event.target.parentElement;
        const link = target ? target.closest('a[href]') : null;
        if (isBlogNavigation(event, link)) armBlogTransition(link);
      }, { capture: true });
      window.addEventListener('resize', applyNavState);
    }

    boot();
