    document.getElementById('nav').innerHTML = AdminCommon.navHTML('blogs');
    document.getElementById('logout_btn').onclick = AdminCommon.logout;

    const metaEl = document.getElementById('meta');
    const titleEl = document.getElementById('title');
    const sectionEl = document.getElementById('section');
    const languageButtonsEl = document.getElementById('language_buttons');
    const markdownEl = document.getElementById('markdown');
    const statusEl = document.getElementById('status');
    const translateLangEl = document.getElementById('translate_lang');
    const translateBtnEl = document.getElementById('translate_btn');
    const translateStatusEl = document.getElementById('translate_status');
    const translateEtaEl = document.getElementById('translate_eta');
    const translateProgressWrapEl = document.getElementById('translate_progress_wrap');
    const translateProgressBarEl = document.getElementById('translate_progress_bar');
    const translateHistoryEl = document.getElementById('translate_history');
    const translateLocaleListEl = document.getElementById('translate_locale_list');

    let state = { blog: null, sections: [], selectedLanguage: 'en' };
    let translateTimer = null;
    let translatePollTimer = null;
    let translateTick = 0;
    let translateStartedAt = 0;
    let translateEtaSeconds = 0;

    function sectionOptions(selected) {
      let html = '<option value="">Unsectioned</option>';
      for (const section of state.sections) {
        const isSelected = section.id === selected;
        html += `<option value="${AdminCommon.escapeHTML(section.id)}" ${isSelected ? 'selected' : ''}>${AdminCommon.escapeHTML(section.name)}</option>`;
      }
      return html;
    }

    function currentBlogID() {
      const path = window.location.pathname;
      const prefix = '/admin/blogs/';
      if (!path.startsWith(prefix)) return '';
      return decodeURIComponent(path.slice(prefix.length));
    }

    async function load() {
      const user = await AdminCommon.requireAuth();
      if (!user) return;

      const blogID = currentBlogID();
      if (!blogID) {
        metaEl.textContent = 'Missing blog ID in URL.';
        return;
      }

      const [catalog, blogResp] = await Promise.all([
        AdminCommon.loadAdminCatalog(),
        AdminCommon.api(`/api/admin/blogs/${blogID}`),
      ]);
      try {
        const localesResp = await AdminCommon.api('/api/locales');
        const locales = Array.isArray(localesResp.locales) ? localesResp.locales : [];
        translateLocaleListEl.innerHTML = locales.map((item) => {
          const code = String(item.code || '').toLowerCase();
          const label = String(item.name || '');
          return `<option value="${AdminCommon.escapeHTML(code)}">${AdminCommon.escapeHTML(label)}</option>`;
        }).join('');
      } catch (_) {}

      const sectionCandidates = Array.isArray(catalog.sections) ? catalog.sections : [];
      state.sections = sectionCandidates.map(s => ({ id: s.id, name: s.name, sort_order: s.sort_order }));
      state.blog = blogResp.blog;
      state.selectedLanguage = (state.blog.languages || []).find(x => x.language === 'en')?.language || state.blog.languages?.[0]?.language || 'en';

      render();
      const data = await refreshTranslationActivity();
      renderActivity(data.activity);
    }

    function render() {
      const blog = state.blog;
      if (!blog) return;
      metaEl.textContent = `${blog.title} · ${(blog.job_id || '').slice(0, 8)} · ${blog.status}`;
      titleEl.value = blog.title || '';
      sectionEl.innerHTML = sectionOptions(blog.section_id || '');

      document.getElementById('toggle_publish_btn').textContent = blog.published ? 'Unpublish' : 'Publish';
      document.getElementById('toggle_delete_btn').textContent = blog.deleted ? 'Restore' : 'Delete';

      const languages = Array.isArray(blog.languages) ? blog.languages : [];
      languageButtonsEl.innerHTML = '';
      if (!languages.find(x => x.language === state.selectedLanguage) && languages.length) {
        state.selectedLanguage = languages[0].language;
      }
      for (const item of languages) {
        const active = item.language === state.selectedLanguage;
        const btn = document.createElement('button');
        btn.className = `rounded-full border px-2.5 py-1 text-xs ${active ? 'border-indigo-300 bg-indigo-50 text-indigo-700' : 'border-slate-300 bg-white text-slate-600 hover:bg-slate-50'}`;
        btn.textContent = item.language;
        btn.onclick = () => {
          state.selectedLanguage = item.language;
          render();
        };
        languageButtonsEl.appendChild(btn);
      }

      const selected = languages.find(x => x.language === state.selectedLanguage) || { markdown: '' };
      markdownEl.value = selected.markdown || '';
      renderTranslationHistory(languages);
    }

    function renderTranslationHistory(languages) {
      const items = (languages || [])
        .filter(x => x.language !== 'en')
        .sort((a, b) => new Date(b.updated_at || 0).getTime() - new Date(a.updated_at || 0).getTime());
      if (!items.length) {
        translateHistoryEl.innerHTML = '<p class=\"text-xs text-slate-500\">No translations yet.</p>';
        return;
      }
      translateHistoryEl.innerHTML = items.map((item) => {
        const ts = item.updated_at ? new Date(item.updated_at).toLocaleString() : '';
        return `<div class=\"rounded border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-600\"><div class=\"flex items-center justify-between gap-2\"><span class=\"font-semibold text-slate-700\">${AdminCommon.escapeHTML(item.language)}</span><span class=\"text-[11px] text-slate-500\">${AdminCommon.escapeHTML(ts)}</span></div></div>`;
      }).join('');
    }

    function estimateTranslateSeconds(markdown) {
      const text = String(markdown || '');
      const words = (text.match(/\\S+/g) || []).length;
      const headings = (text.match(/^#{1,6}\\s+/gm) || []).length;
      const codeBlocks = Math.floor((text.match(/```/g) || []).length / 2);
      const links = (text.match(/\\[[^\\]]+\\]\\([^)]+\\)/g) || []).length;
      const estimated = 7 + (words / 22) + (headings * 0.6) + (codeBlocks * 2.8) + (links * 0.25);
      return Math.max(6, Math.min(300, Math.round(estimated)));
    }

    function stopTranslateProgress() {
      if (translateTimer) {
        clearInterval(translateTimer);
        translateTimer = null;
      }
      translateProgressWrapEl.classList.add('hidden');
      translateProgressBarEl.style.width = '0%';
      translateEtaEl.textContent = '';
      translateTick = 0;
      translateStartedAt = 0;
      translateEtaSeconds = 0;
      translateBtnEl.disabled = false;
      translateBtnEl.classList.remove('opacity-60', 'cursor-not-allowed');
      translateLangEl.disabled = false;
      document.querySelectorAll('[data-lang-quick]').forEach(btn => {
        btn.disabled = false;
        btn.classList.remove('opacity-60', 'cursor-not-allowed');
      });
    }

    function stopTranslatePolling() {
      if (translatePollTimer) {
        clearInterval(translatePollTimer);
        translatePollTimer = null;
      }
    }

    async function refreshTranslationActivity() {
      if (!state.blog?.job_id) return { activity: [] };
      try {
        return await AdminCommon.api(`/api/jobs/${state.blog.job_id}/translations`);
      } catch (_) {
        return { activity: [] };
      }
    }

    function renderActivity(items) {
      const list = Array.isArray(items) ? items : [];
      if (!list.length) return;
      const latest = list[0];
      const lang = latest.language || '';
      if (latest.status === 'running') {
        translateStatusEl.textContent = `Translation running: ${lang}`;
        return;
      }
      if (latest.status === 'failed') {
        translateStatusEl.textContent = `Translation failed (${lang}): ${latest.error_message || 'unknown error'}`;
        return;
      }
      if (latest.status === 'completed') {
        translateStatusEl.textContent = `Translation completed: ${lang}`;
      }
    }

    function startTranslateProgress(lang, markdown) {
      stopTranslateProgress();
      translateBtnEl.disabled = true;
      translateBtnEl.classList.add('opacity-60', 'cursor-not-allowed');
      translateLangEl.disabled = true;
      document.querySelectorAll('[data-lang-quick]').forEach(btn => {
        btn.disabled = true;
        btn.classList.add('opacity-60', 'cursor-not-allowed');
      });
      translateProgressWrapEl.classList.remove('hidden');
      translateEtaSeconds = estimateTranslateSeconds(markdown);
      translateStartedAt = Date.now();
      translateTick = 0;
      translateEtaEl.textContent = `Estimated ~${translateEtaSeconds}s based on markdown size`;
      translateTimer = setInterval(() => {
        translateTick = (translateTick + 1) % 4;
        const dots = '.'.repeat(translateTick).padEnd(3, ' ');
        const elapsed = Math.floor((Date.now() - translateStartedAt) / 1000);
        const pct = Math.max(4, Math.min(95, Math.round((elapsed / Math.max(1, translateEtaSeconds)) * 100)));
        translateProgressBarEl.style.width = `${pct}%`;
        translateStatusEl.textContent = `Translating to ${lang}${dots} (${elapsed}s elapsed)`;
      }, 500);
    }

    document.getElementById('save_meta_btn').onclick = async () => {
      if (!state.blog) return;
      statusEl.textContent = 'Saving metadata...';
      try {
        await AdminCommon.api(`/api/admin/blogs/${state.blog.id}`, {
          method: 'PUT',
          body: JSON.stringify({ title: titleEl.value, section_id: sectionEl.value }),
        });
        statusEl.textContent = 'Metadata saved';
        await load();
      } catch (err) {
        statusEl.textContent = `Save failed: ${err.message}`;
      }
    };

    document.getElementById('toggle_publish_btn').onclick = async () => {
      if (!state.blog) return;
      statusEl.textContent = state.blog.published ? 'Unpublishing...' : 'Publishing...';
      try {
        await AdminCommon.api(`/api/admin/blogs/${state.blog.id}/publish`, {
          method: 'PUT',
          body: JSON.stringify({ published: !state.blog.published }),
        });
        statusEl.textContent = state.blog.published ? 'Unpublished' : 'Published';
        await load();
      } catch (err) {
        statusEl.textContent = `Publish failed: ${err.message}`;
      }
    };

    document.getElementById('toggle_delete_btn').onclick = async () => {
      if (!state.blog) return;
      try {
        if (state.blog.deleted) {
          statusEl.textContent = 'Restoring...';
          await AdminCommon.api(`/api/admin/blogs/${state.blog.id}/restore`, { method: 'POST' });
          statusEl.textContent = 'Restored';
        } else {
          if (!confirm(`Delete blog "${state.blog.title}"?`)) return;
          statusEl.textContent = 'Deleting...';
          await AdminCommon.api(`/api/admin/blogs/${state.blog.id}`, { method: 'DELETE' });
          statusEl.textContent = 'Deleted';
        }
        await load();
      } catch (err) {
        statusEl.textContent = `Delete/restore failed: ${err.message}`;
      }
    };

    document.getElementById('save_content_btn').onclick = async () => {
      if (!state.blog) return;
      statusEl.textContent = 'Saving content...';
      try {
        await AdminCommon.api(`/api/admin/blogs/${state.blog.id}/content`, {
          method: 'PUT',
          body: JSON.stringify({ language: state.selectedLanguage, markdown: markdownEl.value }),
        });
        statusEl.textContent = 'Content saved';
        await load();
      } catch (err) {
        statusEl.textContent = `Save failed: ${err.message}`;
      }
    };

    document.getElementById('clear_override_btn').onclick = async () => {
      if (!state.blog) return;
      statusEl.textContent = 'Clearing override...';
      try {
        await AdminCommon.api(`/api/admin/blogs/${state.blog.id}/content`, {
          method: 'PUT',
          body: JSON.stringify({ language: state.selectedLanguage, markdown: '' }),
        });
        statusEl.textContent = 'Override cleared';
        await load();
      } catch (err) {
        statusEl.textContent = `Clear failed: ${err.message}`;
      }
    };

    translateBtnEl.onclick = async () => {
      if (!state.blog) return;
      const lang = String(translateLangEl.value || '').trim().toLowerCase();
      if (!lang) {
        translateStatusEl.textContent = 'Language is required (example: es).';
        return;
      }
      const languages = Array.isArray(state.blog.languages) ? state.blog.languages : [];
      const source = languages.find(x => x.language === 'en') || languages[0] || { markdown: '' };
      startTranslateProgress(lang, source.markdown || '');
      try {
        await AdminCommon.api(`/api/jobs/${state.blog.job_id}/translate`, {
          method: 'POST',
          body: JSON.stringify({ language: lang }),
        });
        translateStatusEl.textContent = `Translation queued: ${lang}`;
        stopTranslatePolling();
        translatePollTimer = setInterval(async () => {
          const data = await refreshTranslationActivity();
          renderActivity(data.activity);
          const current = (Array.isArray(data.activity) ? data.activity : []).find(x => x.language === lang);
          if (!current) return;
          if (current.status === 'running') return;
          if (current.status === 'completed') {
            translateProgressBarEl.style.width = '100%';
            state.selectedLanguage = lang;
            stopTranslatePolling();
            await load();
            stopTranslateProgress();
            return;
          }
          if (current.status === 'failed') {
            stopTranslatePolling();
            stopTranslateProgress();
          }
        }, 1500);
      } catch (err) {
        translateStatusEl.textContent = `Translate failed: ${err.message}`;
        stopTranslateProgress();
      }
    };

    document.querySelectorAll('[data-lang-quick]').forEach(btn => {
      btn.onclick = () => {
        translateLangEl.value = btn.getAttribute('data-lang-quick') || '';
      };
    });

    load();
