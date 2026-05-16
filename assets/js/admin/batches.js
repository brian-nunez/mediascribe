    document.getElementById('nav').innerHTML = AdminCommon.navHTML('batches');
    const state = { sections: [], stagedItems: [], channelItems: [], channelSelected: new Set(), existingBlogs: [] };

    function esc(s) {
      return String(s || '').replace(/[&<>\"']/g, (m) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[m]));
    }

    async function loadSections() {
      const out = await AdminCommon.api('/api/admin/sections');
      state.sections = Array.isArray(out.sections) ? out.sections : [];
      const options = '<option value="">Unsectioned</option>' + state.sections.map(s => `<option value="${esc(s.id)}">${esc(s.name)}</option>`).join('');
      document.getElementById('item_section').innerHTML = options;
      document.getElementById('channel_section').innerHTML = options;
    }

    async function loadCatalogTitles() {
      const out = await AdminCommon.api('/api/admin/catalog');
      state.existingBlogs = Array.isArray(out.blogs) ? out.blogs : [];
    }

    function titleKey(title, sectionID) {
      return `${String(sectionID || '').trim().toLowerCase()}::${String(title || '').trim().toLowerCase()}`;
    }

    function duplicateWarningsForStagedItems() {
      const warnings = [];
      const existing = new Set();
      for (const b of state.existingBlogs) {
        if (b && !b.deleted) {
          existing.add(titleKey(b.title, b.section_id));
        }
      }
      const stagedSeen = new Set();
      for (let i = 0; i < state.stagedItems.length; i++) {
        const it = state.stagedItems[i];
        const t = String(it.title || '').trim();
        if (!t) continue;
        const k = titleKey(t, it.section_id);
        if (existing.has(k)) {
          warnings.push(`Item #${i + 1}: "${t}" already exists in this section.`);
        }
        if (stagedSeen.has(k)) {
          warnings.push(`Item #${i + 1}: "${t}" is duplicated within this batch for the same section.`);
        }
        stagedSeen.add(k);
      }
      return warnings;
    }

    function renderTitleWarnings() {
      const el = document.getElementById('title_warning');
      const warnings = duplicateWarningsForStagedItems();
      if (!warnings.length) {
        el.classList.add('hidden');
        el.innerHTML = '';
        return warnings;
      }
      el.classList.remove('hidden');
      el.innerHTML = `<p class="font-semibold">Title conflict warning</p><ul class="mt-1 list-disc pl-4">${warnings.map(w => `<li>${esc(w)}</li>`).join('')}</ul>`;
      return warnings;
    }

    function renderStagedItems() {
      const el = document.getElementById('batch_items');
      if (!state.stagedItems.length) {
        el.innerHTML = '<p class="text-slate-500">No items added yet.</p>';
        return;
      }
      el.innerHTML = state.stagedItems.map((it, i) => {
        const section = state.sections.find(s => s.id === it.section_id);
        return `<div class="mb-2 rounded border border-slate-200 p-2">
          <div class="flex items-center justify-between"><strong>${i + 1}. ${esc(it.title || 'Untitled')}</strong><button data-remove="${i}" class="text-xs text-red-600">Remove</button></div>
          <div class="text-xs text-slate-600">${esc(it.source_type)} · ${esc(it.source_url || it.source_path)}</div>
          <div class="text-xs text-slate-500">Section: ${esc(section ? section.name : 'Unsectioned')}</div>
        </div>`;
      }).join('');
      el.querySelectorAll('button[data-remove]').forEach(btn => btn.onclick = () => {
        state.stagedItems.splice(Number(btn.dataset.remove), 1);
        renderStagedItems();
        renderTitleWarnings();
      });
    }

    function renderChannelItems() {
      const el = document.getElementById('channel_items');
      if (!state.channelItems.length) {
        el.innerHTML = '<p class="p-2 text-xs text-slate-500">No channel videos loaded.</p>';
        return;
      }
      el.innerHTML = state.channelItems.map((it, idx) => {
        const checked = state.channelSelected.has(idx) ? 'checked' : '';
        return `<label class="flex cursor-pointer items-start gap-2 border-b border-slate-100 px-2 py-2 text-xs">
          <input type="checkbox" data-idx="${idx}" ${checked} class="mt-0.5 h-4 w-4 rounded border-slate-300" />
          <span>
            <span class="block font-medium text-slate-800">${esc(it.title || it.id || 'Untitled')}</span>
            <span class="block text-slate-500">${esc(it.url)}</span>
          </span>
        </label>`;
      }).join('');
      el.querySelectorAll('input[type="checkbox"][data-idx]').forEach(cb => {
        cb.onchange = () => {
          const idx = Number(cb.dataset.idx);
          if (cb.checked) state.channelSelected.add(idx); else state.channelSelected.delete(idx);
        };
      });
    }

    async function runChannelFetch(isRetry) {
      const statusEl = document.getElementById('channel_status');
      const url = document.getElementById('channel_url').value.trim();
      const limit = Number(document.getElementById('channel_limit').value || 50);
      if (!url) {
        statusEl.textContent = 'Channel URL is required.';
        return;
      }
      statusEl.textContent = isRetry ? 'Retrying fetch...' : 'Fetching videos...';
      let lastErr = null;
      for (let attempt = 1; attempt <= 3; attempt++) {
        try {
          const out = await AdminCommon.api('/api/admin/channel/videos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url, limit }),
          });
          state.channelItems = Array.isArray(out.items) ? out.items : [];
          state.channelSelected = new Set();
          renderChannelItems();
          statusEl.textContent = `Loaded ${state.channelItems.length} videos.`;
          return;
        } catch (err) {
          lastErr = err;
          statusEl.textContent = `Fetch attempt ${attempt}/3 failed...`;
          await new Promise((resolve) => setTimeout(resolve, attempt * 700));
        }
      }
      statusEl.textContent = (lastErr && (lastErr.message || String(lastErr))) || 'Failed to fetch videos.';
    }

    function prettyDate(v) {
      if (!v) return '-';
      const d = new Date(v);
      if (Number.isNaN(d.getTime())) return v;
      return d.toLocaleString();
    }

    async function loadBatches() {
      const out = await AdminCommon.api('/api/admin/batches');
      const jobsOut = await AdminCommon.api('/api/jobs');
      const jobs = Array.isArray(jobsOut) ? jobsOut : [];
      const jobsByID = new Map(jobs.map(j => [j.id, j]));

      const list = Array.isArray(out.batches) ? out.batches : [];
      const wrap = document.getElementById('batches');
      if (!list.length) {
        wrap.innerHTML = '<p class="text-sm text-slate-500">No batches yet.</p>';
        return;
      }

      const details = await Promise.all(list.map(async (b) => {
        try {
          return await AdminCommon.api(`/api/admin/batches/${encodeURIComponent(b.id)}`);
        } catch (_) {
          return { batch: b, items: [] };
        }
      }));

      wrap.innerHTML = details.map((d) => {
        const b = d.batch;
        const items = Array.isArray(d.items) ? d.items : [];
        const status = String(b.status || '').toLowerCase();
        let actionHTML = '';
        if (status === 'queued' || status === 'failed') {
          actionHTML = `<button data-start="${esc(b.id)}" class="rounded border border-slate-300 px-2 py-1 text-xs hover:bg-slate-100">Start</button>`;
        } else if (status === 'running' || status === 'waiting') {
          actionHTML = `<span class="rounded border border-blue-200 bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700">In Progress</span>`;
        } else if (status === 'complete') {
          actionHTML = `<span class="rounded border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700">Completed</span>`;
        } else {
          actionHTML = `<span class="rounded border border-slate-200 bg-slate-50 px-2 py-1 text-xs font-medium text-slate-600">${esc(b.status || 'unknown')}</span>`;
        }
        return `<div class="rounded-lg border border-slate-200 p-3">
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="font-semibold">${esc(b.name)}</p>
              <p class="text-xs text-slate-500">${esc(b.status)} · delay ${b.delay_seconds}s · ${b.processed_items}/${b.total_items}</p>
              <p class="text-xs text-slate-500">next: ${esc(prettyDate(b.next_run_at))}</p>
              ${b.last_error ? `<p class="text-xs text-red-600">${esc(b.last_error)}</p>` : ''}
            </div>
            ${actionHTML}
          </div>
          <div class="mt-2 max-h-56 overflow-y-auto rounded border border-slate-100">
            ${items.map(it => {
              const job = it.job_id ? jobsByID.get(it.job_id) : null;
              const stage = job?.current_stage ? ` · ${esc(job.current_stage)}` : '';
              const jobStatus = job?.status ? ` (${esc(job.status)})` : '';
              return `<div class="border-b border-slate-100 px-2 py-1 text-xs">
                <span class="font-medium">#${it.item_index + 1}</span> ${esc(it.title || 'Untitled')} · ${esc(it.status)}${jobStatus}${stage}
                ${it.job_id ? `<span class="text-slate-500"> · ${esc(it.job_id.slice(0, 8))}</span>` : ''}
                ${it.error_message ? `<div class="text-[11px] text-red-600">${esc(it.error_message)}</div>` : ''}
              </div>`;
            }).join('')}
          </div>
        </div>`;
      }).join('');

      wrap.querySelectorAll('button[data-start]').forEach(btn => {
        btn.onclick = async () => {
          await AdminCommon.api(`/api/admin/batches/${encodeURIComponent(btn.dataset.start)}/start`, { method: 'POST' });
          await loadBatches();
        };
      });
    }

    document.getElementById('add_item').onclick = () => {
      const sourceType = document.getElementById('item_source_type').value;
      const source = document.getElementById('item_source').value.trim();
      if (!source) return;
      state.stagedItems.push({
        source_type: sourceType,
        source_url: sourceType === 'url' ? source : '',
        source_path: sourceType === 'path' ? source : '',
        title: document.getElementById('item_title').value.trim(),
        section_id: document.getElementById('item_section').value.trim(),
        main_model: document.getElementById('item_main_model').value.trim(),
      });
      document.getElementById('item_source').value = '';
      document.getElementById('item_title').value = '';
      document.getElementById('item_main_model').value = '';
      renderStagedItems();
      renderTitleWarnings();
    };

    document.getElementById('fetch_channel').onclick = () => runChannelFetch(false);
    document.getElementById('retry_fetch_channel').onclick = () => runChannelFetch(true);
    document.getElementById('select_all_channel').onclick = () => {
      state.channelSelected = new Set(state.channelItems.map((_, i) => i));
      renderChannelItems();
    };
    document.getElementById('clear_channel_selection').onclick = () => {
      state.channelSelected = new Set();
      renderChannelItems();
    };

    document.getElementById('add_selected_channel').onclick = () => {
      const sectionID = document.getElementById('channel_section').value.trim();
      const mainModel = document.getElementById('channel_main_model').value.trim();
      const selected = Array.from(state.channelSelected.values()).sort((a, b) => a - b);
      if (!selected.length) {
        document.getElementById('channel_status').textContent = 'Select at least one video.';
        return;
      }
      for (const idx of selected) {
        const it = state.channelItems[idx];
        if (!it || !it.url) continue;
        state.stagedItems.push({
          source_type: 'url',
          source_url: it.url,
          source_path: '',
          title: (it.title || '').trim(),
          section_id: sectionID,
          main_model: mainModel,
        });
      }
      renderStagedItems();
      renderTitleWarnings();
      document.getElementById('channel_status').textContent = `Added ${selected.length} videos to batch items.`;
    };

    document.getElementById('create_batch').onclick = async () => {
      const statusEl = document.getElementById('create_status');
      statusEl.textContent = '';
      if (!state.stagedItems.length) {
        statusEl.textContent = 'Add at least one item.';
        return;
      }
      try {
        await loadCatalogTitles();
        const warnings = renderTitleWarnings();
        if (warnings.length) {
          statusEl.textContent = 'Resolve title conflicts before starting this batch.';
          return;
        }
        const out = await AdminCommon.api('/api/admin/batches', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: document.getElementById('batch_name').value.trim() || 'New Batch',
            delay: document.getElementById('batch_delay').value,
            items: state.stagedItems,
            auto_start: true,
          }),
        });
        statusEl.textContent = `Batch created: ${out.batch.id}`;
        state.stagedItems = [];
        renderStagedItems();
        renderTitleWarnings();
        await loadBatches();
      } catch (err) {
        statusEl.textContent = err.message || String(err);
      }
    };

    document.getElementById('refresh').onclick = loadBatches;

    async function boot() {
      await AdminCommon.requireAuth();
      await loadSections();
      await loadCatalogTitles();
      renderStagedItems();
      renderTitleWarnings();
      renderChannelItems();
      await loadBatches();
      setInterval(loadBatches, 5000);
    }
    boot();
