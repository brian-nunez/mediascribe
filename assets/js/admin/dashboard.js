    document.getElementById('nav').innerHTML = AdminCommon.navHTML('dashboard');
    document.getElementById('logout_btn').onclick = AdminCommon.logout;
    const STAGES = [
      'create_job',
      'download_or_copy_video',
      'extract_audio',
      'transcribe_audio',
      'chunk_transcript',
      'generate_embeddings',
      'analyze_chunks',
      'create_outline',
      'write_draft',
      'refine_markdown',
      'optional_translate_markdown',
      'mark_complete',
    ];
    const FRIENDLY_STAGE = {
      create_job: 'Create Job',
      download_or_copy_video: 'Download/Copy Video',
      extract_audio: 'Extract Audio',
      transcribe_audio: 'Transcribe Audio',
      chunk_transcript: 'Chunk Transcript',
      generate_embeddings: 'Generate Embeddings',
      analyze_chunks: 'Analyze Chunks',
      create_outline: 'Create Outline',
      write_draft: 'Write Draft',
      refine_markdown: 'Refine Markdown',
      optional_translate_markdown: 'Translate Markdown',
      mark_complete: 'Complete',
    };
    let selectedJobID = '';
    let jobsPollTimer = null;
    let embeddingsPollTimer = null;
    let statsPollTimer = null;

    function stageStateFor(job, stage, stageIndex, currentIndex) {
      if (job.status === 'complete') return 'done';
      if (job.status === 'failed' && job.current_stage === stage) return 'failed';
      if (job.status === 'running' && job.current_stage === stage) return 'running';
      if (currentIndex >= 0 && stageIndex < currentIndex) return 'done';
      return 'pending';
    }

    function badgeClass(status) {
      if (status === 'complete') return 'border-emerald-300 bg-emerald-50 text-emerald-700';
      if (status === 'running') return 'border-blue-300 bg-blue-50 text-blue-700';
      if (status === 'failed') return 'border-red-300 bg-red-50 text-red-700';
      return 'border-slate-300 bg-slate-50 text-slate-600';
    }

    function renderJobsList(jobs) {
      const listEl = document.getElementById('job_list');
      listEl.innerHTML = '';
      if (!jobs.length) {
        listEl.innerHTML = '<p class=\"rounded-lg border border-dashed border-slate-300 p-3 text-sm text-slate-500\">No jobs yet.</p>';
        document.getElementById('job_detail').textContent = 'No jobs to display.';
        return;
      }

      if (!selectedJobID) selectedJobID = jobs[0].id;
      if (!jobs.find(j => j.id === selectedJobID)) selectedJobID = jobs[0].id;

      for (const job of jobs) {
        const selected = job.id === selectedJobID;
        const row = document.createElement('button');
        row.type = 'button';
        row.className = `w-full rounded-lg border p-2 text-left transition ${selected ? 'border-slate-900 bg-slate-50' : 'border-slate-200 bg-white hover:border-slate-300'}`;
        row.innerHTML = `
          <div class=\"flex items-center justify-between gap-2\">
            <span class=\"text-xs font-semibold text-slate-800\">${AdminCommon.escapeHTML(job.id.slice(0, 8))}</span>
            <span class=\"rounded border px-1.5 py-0.5 text-[10px] ${badgeClass(job.status)}\">${AdminCommon.escapeHTML(job.status)}</span>
          </div>
          <p class=\"mt-1 text-xs text-slate-500\">${AdminCommon.escapeHTML(FRIENDLY_STAGE[job.current_stage] || job.current_stage || '-')}</p>
          <p class=\"mt-1 text-[11px] text-slate-500\">${AdminCommon.escapeHTML(job.main_model || '')}</p>
        `;
        row.onclick = () => {
          selectedJobID = job.id;
          renderJobsList(jobs);
          renderJobDetail(job);
        };
        listEl.appendChild(row);
      }

      const selectedJob = jobs.find(j => j.id === selectedJobID) || jobs[0];
      renderJobDetail(selectedJob);
    }

    function renderJobDetail(job) {
      const detailEl = document.getElementById('job_detail');
      if (!job) {
        detailEl.textContent = 'Select a job to see detailed stage progress.';
        return;
      }

      const currentIndex = STAGES.indexOf(job.current_stage || '');
      const stageRows = STAGES.map((stage, idx) => {
        const st = stageStateFor(job, stage, idx, currentIndex);
        let dot = 'bg-slate-300';
        let text = 'text-slate-500';
        if (st === 'done') { dot = 'bg-emerald-500'; text = 'text-emerald-700'; }
        if (st === 'running') { dot = 'bg-blue-500'; text = 'text-blue-700'; }
        if (st === 'failed') { dot = 'bg-red-500'; text = 'text-red-700'; }
        return `<div class=\"flex items-center gap-2\"><span class=\"h-2.5 w-2.5 rounded-full ${dot}\"></span><span class=\"text-xs ${text}\">${AdminCommon.escapeHTML(FRIENDLY_STAGE[stage] || stage)}</span></div>`;
      }).join('');

      const errorBlock = job.error_message
        ? `<div class=\"mt-3 rounded-lg border border-red-200 bg-red-50 p-2\"><p class=\"text-xs font-semibold text-red-700\">Error</p><p class=\"mt-1 whitespace-pre-wrap text-xs text-red-700\">${AdminCommon.escapeHTML(job.error_message)}</p></div>`
        : '';

      const src = job.source_url || job.source_path || '';
      const srcHTML = src ? AdminCommon.sourceLinkHTML(job) : '<span class=\"text-slate-500\">No source</span>';

      detailEl.innerHTML = `
        <div class=\"flex flex-wrap items-center justify-between gap-2\">
          <p class=\"text-sm font-semibold text-slate-900\">Job ${AdminCommon.escapeHTML(job.id)}</p>
          <span class=\"rounded border px-2 py-0.5 text-xs ${badgeClass(job.status)}\">${AdminCommon.escapeHTML(job.status)}</span>
        </div>
        <p class=\"mt-2 text-xs text-slate-500\">Current stage: ${AdminCommon.escapeHTML(FRIENDLY_STAGE[job.current_stage] || job.current_stage || '-')}</p>
        <div class=\"mt-2 rounded-lg border border-slate-200 bg-white p-2\">
          <p class=\"text-[11px] font-semibold uppercase tracking-wide text-slate-500\">Source</p>
          <div class=\"mt-1 text-xs\">${srcHTML}</div>
        </div>
        <div class=\"mt-3 grid gap-1.5 rounded-lg border border-slate-200 bg-white p-2\">${stageRows}</div>
        ${errorBlock}
      `;
    }

    async function loadJobs() {
      const jobs = await AdminCommon.api('/api/jobs');
      return Array.isArray(jobs) ? jobs : [];
    }

    function renderEmbeddingStatus(status) {
      const statusEl = document.getElementById('rebuild_embeddings_status');
      const metaEl = document.getElementById('rebuild_embeddings_meta');
      const st = status || {};
      if (st.running) {
        statusEl.textContent = `Running: ${st.processed_jobs || 0}/${st.total_jobs || 0} jobs`;
      } else if (st.error_message) {
        statusEl.textContent = `Failed: ${st.error_message}`;
      } else if (st.completed_at) {
        statusEl.textContent = 'Completed';
      } else {
        statusEl.textContent = 'Idle';
      }
      const started = st.started_at ? new Date(st.started_at).toLocaleString() : '-';
      const updated = st.updated_at ? new Date(st.updated_at).toLocaleString() : '-';
      metaEl.textContent = `Chunk embeddings rebuilt: ${st.chunk_embeddings_rebuilt || 0} · Output embeddings rebuilt: ${st.output_embeddings_rebuilt || 0} · Started: ${started} · Updated: ${updated}`;
    }

    async function loadEmbeddingStatus() {
      const out = await AdminCommon.api('/api/admin/embeddings/rebuild');
      renderEmbeddingStatus(out.status || {});
    }

    async function boot() {
      const user = await AdminCommon.requireAuth();
      if (!user) return;
      await loadStats();
      const jobs = await loadJobs();
      renderJobsList(jobs);
      await loadEmbeddingStatus();
    }

    async function loadStats() {
      const statsOut = await AdminCommon.api('/api/admin/stats');
      const stats = statsOut.stats || {};
      document.getElementById('blogs_count').textContent = String(stats.blogs_count || 0);
      document.getElementById('sections_count').textContent = String(stats.sections_count || 0);
      document.getElementById('published_count').textContent = String(stats.published_count || 0);
    }

    document.getElementById('create_btn').onclick = async () => {
      const sourceType = document.getElementById('source_type').value;
      const sourceValue = document.getElementById('source_value').value.trim();
      const mainModel = document.getElementById('main_model').value.trim();
      const status = document.getElementById('create_status');
      if (!sourceValue) {
        status.textContent = 'Source is required.';
        return;
      }
      const payload = { source_type: sourceType };
      if (sourceType === 'url') payload.source_url = sourceValue;
      if (sourceType === 'path') payload.source_path = sourceValue;
      if (mainModel) payload.main_model = mainModel;
      status.textContent = 'Creating...';
      try {
        const out = await AdminCommon.api('/api/jobs', { method: 'POST', body: JSON.stringify(payload) });
        status.textContent = `Created ${out.job_id.slice(0, 8)}`;
        selectedJobID = out.job_id;
        const jobs = await loadJobs();
        renderJobsList(jobs);
      } catch (err) {
        status.textContent = `Create failed: ${err.message}`;
      }
    };

    document.getElementById('refresh_jobs_btn').onclick = async () => {
      const jobs = await loadJobs();
      renderJobsList(jobs);
    };

    document.getElementById('rebuild_embeddings_btn').onclick = async () => {
      const statusEl = document.getElementById('rebuild_embeddings_status');
      statusEl.textContent = 'Starting...';
      try {
        const out = await AdminCommon.api('/api/admin/embeddings/rebuild', { method: 'POST' });
        renderEmbeddingStatus(out.status || {});
      } catch (err) {
        statusEl.textContent = `Start failed: ${err.message}`;
      }
    };

    document.getElementById('sync_artifacts_btn').onclick = async () => {
      const statusEl = document.getElementById('sync_artifacts_status');
      statusEl.textContent = 'Syncing...';
      try {
        const out = await AdminCommon.api('/api/admin/artifacts/sync', { method: 'POST' });
        const r = out.result || {};
        statusEl.textContent = `Scanned ${r.blogs_scanned || 0}, updated ${r.updated || 0}`;
      } catch (err) {
        statusEl.textContent = `Sync failed: ${err.message}`;
      }
    };

    boot();
    jobsPollTimer = setInterval(async () => {
      try {
        const jobs = await loadJobs();
        renderJobsList(jobs);
      } catch (_) {}
    }, 5000);
    embeddingsPollTimer = setInterval(async () => {
      try {
        await loadEmbeddingStatus();
      } catch (_) {}
    }, 4000);
    statsPollTimer = setInterval(async () => {
      try {
        await loadStats();
      } catch (_) {}
    }, 5000);
