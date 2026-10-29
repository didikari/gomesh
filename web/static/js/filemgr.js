// filemgr.js - Remote File Explorer (Browse, Download, Upload, Mkdir, Delete)

let activeFileDeviceID = null;
let currentLoadedPath = '';
let currentFileEntries = [];

function openFilesModal(deviceID, hostname) {
  activeFileDeviceID = deviceID;
  const modal = document.getElementById('files-modal');
  const title = document.getElementById('files-modal-title');
  modal.classList.remove('hidden');
  title.innerText = `File Explorer: ${hostname}`;
  loadDir('');
}

function closeFilesModal() {
  document.getElementById('files-modal').classList.add('hidden');
  activeFileDeviceID = null;
}

async function loadDir(path) {
  if (!activeFileDeviceID) return;
  const tbody = document.getElementById('files-tbody');
  tbody.innerHTML = `
    <tr>
      <td colspan="5" class="py-12 text-center text-slate-500">
        <div class="flex flex-col items-center justify-center gap-2">
          <i data-lucide="loader-2" class="w-6 h-6 animate-spin text-amber-400"></i>
          <p>Memuat daftar file...</p>
        </div>
      </td>
    </tr>
  `;
  if (window.lucide) lucide.createIcons();

  try {
    const query = path ? `?path=${encodeURIComponent(path)}` : '';
    const res = await fetch(`/api/devices/${activeFileDeviceID}/files${query}`);
    if (!res.ok) {
      const err = await res.text();
      throw new Error(err);
    }
    const data = await res.json();
    currentLoadedPath = data.path;
    document.getElementById('path-input').value = data.path;
    document.getElementById('files-current-path').innerText = data.path;
    renderFileList(data.entries || []);
  } catch (err) {
    tbody.innerHTML = `
      <tr>
        <td colspan="5" class="py-8 text-center text-rose-400">
          <div class="flex flex-col items-center justify-center gap-1">
            <i data-lucide="alert-triangle" class="w-6 h-6 text-rose-500"></i>
            <p class="font-semibold">Gagal memuat direktori</p>
            <p class="text-xs text-slate-500">${err.message}</p>
          </div>
        </td>
      </tr>
    `;
    if (window.lucide) lucide.createIcons();
  }
}

function renderFileList(entries) {
  currentFileEntries = entries || [];
  const tbody = document.getElementById('files-tbody');
  document.getElementById('files-count-label').innerText = `${entries.length} items`;

  if (entries.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="5" class="py-8 text-center text-slate-500">
          Folder ini kosong.
        </td>
      </tr>
    `;
    return;
  }

  tbody.innerHTML = entries.map((e, idx) => {
    const isDir = e.is_dir;
    const icon = isDir
      ? `<i data-lucide="folder" class="w-4 h-4 text-amber-400 fill-amber-400/20"></i>`
      : `<i data-lucide="file-text" class="w-4 h-4 text-slate-400"></i>`;

    const nameClick = isDir
      ? `onclick="loadDirByIndex(${idx})" class="cursor-pointer text-amber-300 hover:text-amber-200 font-semibold flex items-center gap-2"`
      : `class="text-slate-200 flex items-center gap-2"`;

    const downloadBtn = !isDir
      ? `<button onclick="downloadFileByIndex(${idx})" class="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-indigo-400 transition-colors" title="Download"><i data-lucide="download" class="w-3.5 h-3.5"></i></button>`
      : '';

    const deleteBtn = `<button onclick="deleteFileByIndex(${idx})" class="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-rose-400 transition-colors" title="Delete"><i data-lucide="trash-2" class="w-3.5 h-3.5"></i></button>`;

    return `
      <tr class="hover:bg-slate-800/40 transition-colors">
        <td class="py-2.5 px-3">
          <div ${nameClick}>
            ${icon}
            <span class="truncate max-w-[150px] sm:max-w-md">${escapeHtml(e.name)}</span>
          </div>
        </td>
        <td class="py-2.5 px-3 font-mono text-[11px] text-slate-400">${isDir ? '-' : formatBytes(e.size)}</td>
        <td class="py-2.5 px-3 font-mono text-[11px] text-slate-500 hidden md:table-cell">${e.mode || '-'}</td>
        <td class="py-2.5 px-3 text-slate-400 text-[11px] hidden sm:table-cell">${new Date(e.mod_time).toLocaleString()}</td>
        <td class="py-2.5 px-3 text-right">
          <div class="flex items-center justify-end gap-1">
            ${downloadBtn}
            ${deleteBtn}
          </div>
        </td>
      </tr>
    `;
  }).join('');

  if (window.lucide) lucide.createIcons();
}

function loadDirByIndex(idx) {
  const item = currentFileEntries[idx];
  if (item && item.is_dir) {
    loadDir(item.path);
  }
}

function downloadFileByIndex(idx) {
  const item = currentFileEntries[idx];
  if (item && !item.is_dir) {
    downloadFile(item.path);
  }
}

function deleteFileByIndex(idx) {
  const item = currentFileEntries[idx];
  if (item) {
    deleteFile(item.path);
  }
}

function escapeQuotes(str) {
  return (str || '')
    .replace(/\\/g, '\\\\')
    .replace(/'/g, "\\'")
    .replace(/"/g, '&quot;');
}

function navigateUp() {
  if (!currentLoadedPath || currentLoadedPath === '/' || currentLoadedPath === '.') return;
  let clean = currentLoadedPath.replace(/\\/g, '/').replace(/\/+$/, '');
  const parts = clean.split('/');
  parts.pop();
  let parent = parts.join('/');
  if (!parent) parent = '/';
  if (/^[a-zA-Z]:$/.test(parent)) parent += '/';
  loadDir(parent);
}

function triggerFileUpload() {
  document.getElementById('hidden-file-input').click();
}

async function handleFileSelected(event) {
  const file = event.target.files[0];
  if (!file) return;

  const status = document.getElementById('upload-status');
  status.innerText = `Mengunggah ${file.name}...`;

  const formData = new FormData();
  formData.append('file', file);

  try {
    const query = currentLoadedPath ? `?path=${encodeURIComponent(currentLoadedPath)}` : '';
    const res = await fetch(`/api/devices/${activeFileDeviceID}/files/upload${query}`, {
      method: 'POST',
      body: formData,
    });

    if (!res.ok) throw new Error(await res.text());
    status.innerText = `Berhasil mengunggah ${file.name}`;
    setTimeout(() => { status.innerText = ''; }, 3000);
    loadDir(currentLoadedPath);
  } catch (err) {
    showToast('Gagal mengunggah file: ' + err.message, 'error');
    status.innerText = '';
  } finally {
    event.target.value = '';
  }
}

function downloadFile(filePath) {
  if (!activeFileDeviceID) return;
  const url = `/api/devices/${activeFileDeviceID}/files/download?path=${encodeURIComponent(filePath)}`;
  window.open(url, '_blank');
}

async function deleteFile(filePath) {
  const ok = await showConfirmModal({
    title: 'Hapus File',
    message: `Hapus "${filePath}"? Tindakan ini tidak dapat dibatalkan.`,
    confirmText: 'Hapus',
    isDanger: true
  });
  if (!ok) return;

  try {
    const res = await fetch(`/api/devices/${activeFileDeviceID}/files/action`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'delete', path: filePath })
    });
    if (!res.ok) throw new Error(await res.text());
    showToast(`File "${filePath}" berhasil dihapus`, 'success');
    loadDir(currentLoadedPath);
  } catch (err) {
    showToast('Gagal menghapus: ' + err.message, 'error');
  }
}

async function createNewFolder() {
  const folderName = await showPromptModal({
    title: 'Buat Folder Baru',
    message: 'Masukkan nama folder baru:',
    placeholder: 'nama-folder'
  });
  if (!folderName || !folderName.trim()) return;
  const trimmed = folderName.trim();
  const fullPath = currentLoadedPath ? `${currentLoadedPath.replace(/\/+$/, '')}/${trimmed}` : trimmed;

  try {
    const res = await fetch(`/api/devices/${activeFileDeviceID}/files/action`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'mkdir', path: fullPath })
    });
    if (!res.ok) throw new Error(await res.text());
    showToast(`Folder "${trimmed}" berhasil dibuat`, 'success');
    loadDir(currentLoadedPath);
  } catch (err) {
    showToast('Gagal membuat folder: ' + err.message, 'error');
  }
}
