    document.getElementById('nav').innerHTML = AdminCommon.navHTML('sections');
    document.getElementById('logout_btn').onclick = AdminCommon.logout;

    async function render() {
      const user = await AdminCommon.requireAuth();
      if (!user) return;
      const data = await AdminCommon.api('/api/admin/sections');
      const list = Array.isArray(data.sections) ? data.sections : [];
      const el = document.getElementById('list');
      el.innerHTML = '';
      if (!list.length) {
        el.innerHTML = '<p class="rounded-lg border border-dashed border-slate-300 p-3 text-sm text-slate-500">No sections yet.</p>';
        return;
      }
      for (const section of list) {
        const row = document.createElement('div');
        row.className = 'rounded-lg border border-slate-200 bg-slate-50 px-2.5 py-2';
        row.innerHTML = `
          <div class="flex items-center gap-2">
            <div>
              <p class="text-sm font-semibold text-slate-800">${AdminCommon.escapeHTML(section.name)}</p>
              <p class="text-[11px] text-slate-500">Order ${AdminCommon.escapeHTML(String(section.sort_order ?? 0))}</p>
            </div>
            <div class="ml-auto flex items-center gap-1.5">
              <button data-rename class="rounded border border-slate-300 bg-white px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100">Rename</button>
              <button data-order class="rounded border border-slate-300 bg-white px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100">Order</button>
              <button data-delete class="rounded border border-red-300 bg-red-50 px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-100">Delete</button>
            </div>
          </div>
        `;
        row.querySelector('[data-rename]').onclick = async () => {
          const name = prompt('New section name:', section.name || '');
          if (name === null) return;
          await AdminCommon.api(`/api/admin/sections/${section.id}`, {
            method: 'PUT',
            body: JSON.stringify({ name, sort_order: Number(section.sort_order || 0) }),
          });
          await render();
        };
        row.querySelector('[data-order]').onclick = async () => {
          const val = prompt('New sort order:', String(section.sort_order ?? 0));
          if (val === null) return;
          await AdminCommon.api(`/api/admin/sections/${section.id}`, {
            method: 'PUT',
            body: JSON.stringify({ name: section.name, sort_order: Number(val || 0) }),
          });
          await render();
        };
        row.querySelector('[data-delete]').onclick = async () => {
          if (!confirm(`Delete section "${section.name}"?`)) return;
          await AdminCommon.api(`/api/admin/sections/${section.id}`, { method: 'DELETE' });
          await render();
        };
        el.appendChild(row);
      }
    }

    document.getElementById('create_btn').onclick = async () => {
      const name = document.getElementById('new_name').value.trim();
      if (!name) return;
      await AdminCommon.api('/api/admin/sections', {
        method: 'POST',
        body: JSON.stringify({ name }),
      });
      document.getElementById('new_name').value = '';
      await render();
    };

    render();
