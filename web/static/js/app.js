// app.js - Main dashboard application logic, device telemetry, polling, and modal shortcuts

let currentUser = null;
let activeToken = 'default-token';
let allDevices = [];
let currentGroupFilter = 'ALL';
let detectedLANHost = '';

// --- ENROLLMENT TOKENS & DOWNLOAD / INSTALL SCRIPT HELPERS ---
async function loadActiveToken() {
  try {
    const res = await fetch('/api/tokens');
    if (res.ok) {
      const tokens = await res.json() || [];
      if (tokens.length > 0) {
        activeToken = tokens[0].token;
        updateInstallCommands();
      }
    }
  } catch (e) {
    console.error('Failed fetching tokens', e);
  }
}

async function generateNewToken() {
  try {
    const desc = await showPromptModal({
      title: 'Buat Token Pendaftaran Baru',
      message: 'Masukkan deskripsi untuk token pendaftaran node (opsional):',
      defaultValue: 'Agent Node Token',
      placeholder: 'Deskripsi token...'
    });
    if (desc === null) return;
    const res = await fetch('/api/tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ description: desc || 'Agent Token', expires_days: 30 })
    });
    if (res.ok) {
      const tok = await res.json();
      activeToken = tok.token;
      updateInstallCommands();
      showToast('Token pendaftaran baru berhasil dibuat!', 'success');
    } else {
      showToast('Hanya admin yang dapat membuat token pendaftaran baru.', 'error');
    }
  } catch (e) {
    showToast('Gagal membuat token: ' + e.message, 'error');
  }
}

function updateInstallCommands() {
  const lanInput = document.getElementById('target-lan-input');
  const pubInput = document.getElementById('target-public-input');
  const targetGroup = (document.getElementById('target-group-input')?.value || 'Default').trim();

  let lanHost = (lanInput && lanInput.value.trim()) || '';
  let pubHost = (pubInput && pubInput.value.trim()) || '';

  // Clean protocol prefixes
  lanHost = lanHost.replace(/^https?:\/\//i, '').replace(/^wss?:\/\//i, '').replace(/\/+$/, '');
  pubHost = pubHost.replace(/^https?:\/\//i, '').replace(/^wss?:\/\//i, '').replace(/\/+$/, '');

  function makeWS(h) {
    if (!h) return '';
    const isSecure = h.includes('.trycloudflare.com') || h.includes('.ngrok') || window.location.protocol === 'https:';
    return `${isSecure ? 'wss:' : 'ws:'}//${h}/ws/agent`;
  }
  function makeHTTP(h) {
    if (!h) return '';
    const isSecure = h.includes('.trycloudflare.com') || h.includes('.ngrok') || window.location.protocol === 'https:';
    return `${isSecure ? 'https:' : 'http:'}//${h}`;
  }

  const lanWS = makeWS(lanHost);
  const pubWS = makeWS(pubHost);
  const lanHTTP = makeHTTP(lanHost || window.location.host);

  // Build endpoints list
  const endpointsList = [];
  if (lanWS) endpointsList.push(lanWS);
  if (pubWS) endpointsList.push(pubWS);
  if (endpointsList.length === 0) endpointsList.push(`ws://${window.location.host}/ws/agent`);

  const endpointsParam = encodeURIComponent(endpointsList.join(','));
  const lanParam = lanWS ? `&lan_server=${encodeURIComponent(lanWS)}` : '';
  const pubParam = pubWS ? `&public_server=${encodeURIComponent(pubWS)}` : '';

  const warnBanner = document.getElementById('localhost-warning-banner');
  if (warnBanner) {
    if ((lanHost.includes('localhost') || lanHost.includes('127.0.0.1')) && !pubHost) {
      warnBanner.classList.remove('hidden');
    } else {
      warnBanner.classList.add('hidden');
    }
  }

  const tokenDisplay = document.getElementById('token-display');
  if (tokenDisplay) tokenDisplay.innerText = activeToken;

  const dlQuery = `token=${encodeURIComponent(activeToken)}&group=${encodeURIComponent(targetGroup)}&endpoints=${endpointsParam}${lanParam}${pubParam}`;

  const winExeBtn = document.getElementById('btn-dl-win-exe');
  if (winExeBtn) winExeBtn.href = `/bin/gomesh-agent.exe?${dlQuery}`;
  const linuxBinBtn = document.getElementById('btn-dl-linux-bin');
  if (linuxBinBtn) linuxBinBtn.href = `/bin/gomesh-agent-linux?${dlQuery}`;
  const macosBinBtn = document.getElementById('btn-dl-macos-bin');
  if (macosBinBtn) macosBinBtn.href = `/bin/gomesh-agent?${dlQuery}`;

  const winBatBtn = document.getElementById('btn-dl-win-bat');
  if (winBatBtn) winBatBtn.href = `/api/download/installer/windows-bat?${dlQuery}`;
  const linuxShBtn = document.getElementById('btn-dl-linux-sh');
  if (linuxShBtn) linuxShBtn.href = `/api/download/installer/linux-sh?${dlQuery}`;
  const macosShBtn = document.getElementById('btn-dl-macos-sh');
  if (macosShBtn) macosShBtn.href = `/api/download/installer/macos-sh?${dlQuery}`;

  const epsFlag = endpointsList.length > 1 ? ` -e "${endpointsList.join(',')}"` : '';
  const cmdLinux = document.getElementById('cmd-linux');
  if (cmdLinux) {
    cmdLinux.innerText = `curl -sSL "${lanHTTP}/scripts/install-linux.sh" | sudo bash -s -- -s "${endpointsList[0]}" -t "${activeToken}" -g "${targetGroup}"${epsFlag}`;
  }

  const cmdWin = document.getElementById('cmd-windows');
  if (cmdWin) {
    cmdWin.innerText = `powershell -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri '${lanHTTP}/scripts/install-windows.ps1' -OutFile install.ps1; .\\install.ps1 -ServerUrl '${endpointsList[0]}' -Token '${activeToken}' -Group '${targetGroup}'"`;
  }

  const cmdMac = document.getElementById('cmd-macos');
  if (cmdMac) {
    cmdMac.innerText = `curl -sSL "${lanHTTP}/scripts/install-macos.sh" | sudo bash -s -- -s "${endpointsList[0]}" -t "${activeToken}" -g "${targetGroup}"${epsFlag}`;
  }
}

function selectInstallTab(os) {
  ['linux', 'windows', 'macos'].forEach(t => {
    const btn = document.getElementById(`tab-btn-${t}`);
    const content = document.getElementById(`tab-content-${t}`);
    if (t === os) {
      if (btn) btn.className = 'px-4 py-2 text-xs font-semibold border-b-2 border-indigo-500 text-indigo-400 inline-flex items-center gap-2';
      if (content) content.classList.remove('hidden');
    } else {
      if (btn) btn.className = 'px-4 py-2 text-xs font-semibold border-b-2 border-transparent text-slate-400 hover:text-slate-200 inline-flex items-center gap-2';
      if (content) content.classList.add('hidden');
    }
  });
  if (window.lucide) lucide.createIcons();
}

async function openInstallModal() {
  const lanInput = document.getElementById('target-lan-input');
  const pubInput = document.getElementById('target-public-input');

  try {
    const res = await fetch('/api/server/network');
    if (res.ok) {
      const netInfo = await res.json();
      if (netInfo.primary_lan_ip) {
        detectedLANHost = `${netInfo.primary_lan_ip}:${netInfo.port || 8080}`;
        const quickLan = document.getElementById('quick-lan-ip');
        if (quickLan) quickLan.innerText = detectedLANHost;
        if (lanInput && (!lanInput.value || lanInput.value.includes('localhost') || lanInput.value.includes('127.0.0.1'))) {
          lanInput.value = detectedLANHost;
        }
      }

      if (netInfo.lan_ips && netInfo.lan_ips.length > 0) {
        const chipsDiv = document.getElementById('lan-ip-chips');
        const chipsContainer = document.getElementById('lan-ip-chips-container');
        if (chipsDiv && chipsContainer) {
          chipsContainer.innerHTML = netInfo.lan_ips.map(ip => {
            const target = `${ip}:${netInfo.port || 8080}`;
            return `<button type="button" onclick="selectSuggestedLAN('${target}')" class="px-1.5 py-0.5 rounded bg-slate-800 hover:bg-emerald-950 hover:text-emerald-300 border border-slate-700 text-slate-300 font-mono transition-colors text-[10px]">${target}</button>`;
          }).join('');
          chipsDiv.classList.remove('hidden');
        }
      }
    }
  } catch (e) {
    console.warn('Network discovery error:', e);
  }

  if (lanInput && !lanInput.value) {
    lanInput.value = window.location.host;
  }

  if (pubInput && !pubInput.value) {
    const host = window.location.host;
    if (!host.includes('localhost') && !host.includes('127.0.0.1') && !host.startsWith('192.168.') && !host.startsWith('10.')) {
      pubInput.value = host;
    }
  }

  updateInstallCommands();
  document.getElementById('install-modal').classList.remove('hidden');
  if (window.lucide) lucide.createIcons();
}

function selectSuggestedLAN(target) {
  const lanInput = document.getElementById('target-lan-input');
  if (lanInput) {
    lanInput.value = target;
    updateInstallCommands();
  }
}

function applyDetectedLAN() {
  if (detectedLANHost) {
    const lanInput = document.getElementById('target-lan-input');
    if (lanInput) {
      lanInput.value = detectedLANHost;
      updateInstallCommands();
    }
  }
}

function closeInstallModal() {
  document.getElementById('install-modal').classList.add('hidden');
}

function copyCmd(elementId) {
  const el = document.getElementById(elementId);
  if (!el) return;
  navigator.clipboard.writeText(el.innerText);
  showToast('Perintah instalasi disalin ke clipboard!', 'success');
}

// --- AUDIT LOGS MODAL ---
async function openAuditsModal() {
  document.getElementById('audits-modal').classList.remove('hidden');
  if (window.lucide) lucide.createIcons();
  const tbody = document.getElementById('audits-tbody');
  tbody.innerHTML = `<tr><td colspan="5" class="py-8 text-center text-slate-500">Memuat audit log...</td></tr>`;

  try {
    const res = await fetch('/api/audits');
    const audits = await res.json() || [];
    if (audits.length === 0) {
      tbody.innerHTML = `<tr><td colspan="5" class="py-8 text-center text-slate-500">Belum ada riwayat audit sesi remote.</td></tr>`;
    } else {
      tbody.innerHTML = audits.map(a => `
        <tr class="hover:bg-slate-950/30">
          <td class="py-2.5 px-3">
            <span class="px-2 py-0.5 rounded font-mono text-[10px] uppercase font-bold ${a.session_type === 'desktop' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-indigo-500/10 text-indigo-400 border border-indigo-500/20'}">
              ${a.session_type}
            </span>
          </td>
          <td class="py-2.5 px-3 font-semibold text-slate-200">${escapeHtml(a.username)}</td>
          <td class="py-2.5 px-3 font-mono text-slate-400">${a.device_id.substring(0, 8)}...</td>
          <td class="py-2.5 px-3 text-slate-400">${new Date(a.started_at).toLocaleString()}</td>
          <td class="py-2.5 px-3 font-mono text-slate-400">${escapeHtml(a.client_ip || '-')}</td>
        </tr>
      `).join('');
    }
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="5" class="py-8 text-center text-rose-400">Gagal memuat log audit: ${escapeHtml(e.message)}</td></tr>`;
  }
}

function closeAuditsModal() {
  document.getElementById('audits-modal').classList.add('hidden');
}

// --- DEVICE GROUPING & LIST ---
function setGroupFilter(group) {
  currentGroupFilter = group;
  renderGroupTabs();
  renderDeviceTable();
}

function renderGroupTabs() {
  const groups = {};
  allDevices.forEach(d => {
    const g = d.group_name || 'Default';
    groups[g] = (groups[g] || 0) + 1;
  });

  const container = document.getElementById('group-tabs-container');
  const allActive = currentGroupFilter === 'ALL';
  let html = `
    <button onclick="setGroupFilter('ALL')" class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-all flex items-center gap-1.5 whitespace-nowrap shrink-0 ${allActive ? 'bg-indigo-600 text-white shadow-sm' : 'bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-800'}">
      <i data-lucide="layers" class="w-3.5 h-3.5"></i> All Devices (${allDevices.length})
    </button>
  `;

  Object.keys(groups).sort().forEach(g => {
    const isActive = currentGroupFilter === g;
    html += `
      <button onclick="setGroupFilter('${escapeQuotes(g)}')" class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-all flex items-center gap-1.5 whitespace-nowrap shrink-0 ${isActive ? 'bg-indigo-600 text-white shadow-sm' : 'bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-800'}">
        <i data-lucide="folder" class="w-3.5 h-3.5 ${isActive ? 'text-white' : 'text-amber-400'}"></i> ${escapeHtml(g)} (${groups[g]})
      </button>
    `;
  });

  container.innerHTML = html;
  const statGroups = document.getElementById('stat-groups');
  if (statGroups) statGroups.innerText = Object.keys(groups).length || 1;
  if (window.lucide) lucide.createIcons();
}

async function changeDeviceGroup(deviceID, currentGroup, hostname) {
  const newGroup = await showPromptModal({
    title: 'Pindahkan Group Perangkat',
    message: `Pindahkan komputer "${hostname}" ke Group:`,
    defaultValue: currentGroup || 'Default',
    placeholder: 'Nama group...'
  });
  if (newGroup === null) return;
  const trimmed = newGroup.trim();
  if (!trimmed) return;

  try {
    const res = await fetch(`/api/devices/${deviceID}/group`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ group: trimmed })
    });
    if (!res.ok) throw new Error('Gagal memperbarui group');
    showToast(`Group untuk "${hostname}" diubah ke "${trimmed}"`, 'success');
    fetchDevices();
  } catch (e) {
    showToast(e.message, 'error');
  }
}

function getOSIcon(os) {
  os = (os || '').toLowerCase();
  if (os.includes('darwin') || os.includes('mac')) {
    return '<span class="inline-flex items-center gap-1 text-slate-200"><i data-lucide="apple" class="w-4 h-4 text-slate-300"></i> macOS</span>';
  } else if (os.includes('windows')) {
    return '<span class="inline-flex items-center gap-1 text-sky-400"><i data-lucide="app-window" class="w-4 h-4"></i> Windows</span>';
  } else if (os.includes('linux')) {
    return '<span class="inline-flex items-center gap-1 text-amber-400"><i data-lucide="terminal" class="w-4 h-4"></i> Linux</span>';
  }
  return `<span class="text-slate-300">${escapeHtml(os)}</span>`;
}

async function fetchDevices() {
  try {
    const res = await fetch('/api/devices');
    if (res.status === 401) {
      window.location.href = '/login';
      return;
    }
    if (!res.ok) throw new Error('Failed to fetch devices');
    allDevices = await res.json() || [];

    let onlineCount = 0;
    let offlineCount = 0;
    allDevices.forEach(d => {
      if (d.status === 'online') onlineCount++; else offlineCount++;
    });

    const statTotal = document.getElementById('stat-total');
    if (statTotal) statTotal.innerText = allDevices.length;
    const statOnline = document.getElementById('stat-online');
    if (statOnline) statOnline.innerText = onlineCount;
    const statOffline = document.getElementById('stat-offline');
    if (statOffline) statOffline.innerText = offlineCount;

    renderGroupTabs();
    renderDeviceTable();
  } catch (err) {
    console.error('Fetch devices failed:', err);
  }
}

function renderDeviceTable() {
  const tbody = document.getElementById('device-tbody');
  const search = (document.getElementById('search-input')?.value || '').toLowerCase().trim();

  const filtered = allDevices.filter(d => {
    const grp = d.group_name || 'Default';
    if (currentGroupFilter !== 'ALL' && grp !== currentGroupFilter) {
      return false;
    }
    if (search) {
      const matchHost = (d.hostname || '').toLowerCase().includes(search);
      const matchIP = (d.internal_ip || '').toLowerCase().includes(search) || (d.public_ip || '').toLowerCase().includes(search);
      const matchGroup = grp.toLowerCase().includes(search);
      const matchOS = (d.os || '').toLowerCase().includes(search);
      return matchHost || matchIP || matchGroup || matchOS;
    }
    return true;
  });

  if (filtered.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="8" class="py-12 text-center text-slate-500">
          <div class="flex flex-col items-center justify-center gap-2">
            <i data-lucide="inbox" class="w-8 h-8 text-slate-600"></i>
            <p class="font-medium text-slate-400">Tidak ada perangkat yang sesuai filter</p>
            <p class="text-xs text-slate-500">${allDevices.length === 0 ? 'Klik "Add Device" atau "Download Agent" untuk mendaftarkan komputer target.' : 'Coba pilih group lain atau bersihkan kata kunci pencarian.'}</p>
          </div>
        </td>
      </tr>
    `;
  } else {
    tbody.innerHTML = filtered.map(d => {
      const isOnline = d.status === 'online';
      const groupName = d.group_name || 'Default';
      const isLAN = (d.connection_mode || '').toUpperCase() === 'LAN';

      const connBadge = isOnline ? (
        isLAN
          ? `<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20" title="Terhubung langsung via jaringan kantor (LAN)"><i data-lucide="wifi" class="w-2.5 h-2.5"></i> LAN</span>`
          : `<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-sky-500/10 text-sky-400 border border-sky-500/20" title="Terhubung dari luar kantor via Internet / Cloudflare"><i data-lucide="globe" class="w-2.5 h-2.5"></i> WAN</span>`
      ) : '';

      const statusBadge = isOnline
        ? `<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"><span class="h-1.5 w-1.5 rounded-full bg-emerald-400 pulse-dot"></span> Online</span>`
        : `<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-slate-800 text-slate-400 border border-slate-700"><span class="h-1.5 w-1.5 rounded-full bg-slate-500"></span> Offline</span>`;

      const cpuBar = isOnline ? `
        <div class="w-28 space-y-1">
          <div class="flex justify-between text-[10px] text-slate-400 font-mono">
            <span>CPU: ${d.cpu_usage.toFixed(1)}%</span>
            <span>RAM: ${d.ram_usage.toFixed(1)}%</span>
          </div>
          <div class="h-1.5 bg-slate-800 rounded-full overflow-hidden flex">
            <div class="bg-indigo-500 h-full" style="width: ${Math.min(d.cpu_usage, 100)}%"></div>
          </div>
        </div>
      ` : `<span class="text-xs text-slate-500">-</span>`;

      return `
        <tr class="hover:bg-slate-900/40 transition-colors">
          <td class="py-3 px-3 sm:px-4">${statusBadge}</td>
          <td class="py-3 px-3 sm:px-4">
            <div class="font-semibold text-white text-xs sm:text-sm flex items-center gap-1.5">
              <span>${escapeHtml(d.hostname)}</span>
            </div>
            <div class="flex items-center gap-2 mt-0.5 text-xs text-slate-400">
              ${getOSIcon(d.os)}
              <span class="sm:hidden text-slate-600">•</span>
              <span class="sm:hidden font-mono text-[11px] text-slate-300">${escapeHtml(d.internal_ip || '')}</span>
            </div>
            <!-- Mobile-only secondary info: Group & CPU badge -->
            <div class="sm:hidden mt-1.5 flex flex-wrap items-center gap-1">
              <button onclick="changeDeviceGroup('${d.id}', '${escapeQuotes(groupName)}', '${escapeQuotes(d.hostname)}')" class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono bg-slate-900 border border-slate-800 text-indigo-300">
                <i data-lucide="tag" class="w-2.5 h-2.5 text-slate-500"></i> ${escapeHtml(groupName)}
              </button>
              ${isOnline ? `<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono bg-slate-900 border border-slate-800 text-slate-400">CPU ${d.cpu_usage.toFixed(0)}%</span>` : ''}
            </div>
          </td>
          <td class="py-3.5 px-4 hidden sm:table-cell">
            <button onclick="changeDeviceGroup('${d.id}', '${escapeQuotes(groupName)}', '${escapeQuotes(d.hostname)}')" title="Klik untuk memindahkan group" class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 hover:border-indigo-500/50 text-xs font-mono text-indigo-300 transition-all group">
              <i data-lucide="tag" class="w-3 h-3 text-slate-500 group-hover:text-indigo-400"></i>
              <span>${escapeHtml(groupName)}</span>
              <i data-lucide="edit-2" class="w-2.5 h-2.5 text-slate-500 opacity-60 group-hover:opacity-100 ml-0.5"></i>
            </button>
          </td>
          <td class="py-3.5 px-4 hidden lg:table-cell font-mono text-xs text-slate-300">${escapeHtml(d.arch)}</td>
          <td class="py-3.5 px-4 hidden md:table-cell font-mono text-xs">
            <div class="flex items-center gap-1.5">
              <span class="text-slate-200">${escapeHtml(d.internal_ip || '-')}</span>
              ${connBadge}
            </div>
            <div class="text-slate-500 text-[10px] truncate max-w-[150px]">${escapeHtml(d.public_ip || '')}</div>
          </td>
          <td class="py-3.5 px-4 hidden sm:table-cell">${cpuBar}</td>
          <td class="py-3.5 px-4 hidden xl:table-cell text-xs text-slate-400">${timeAgo(d.last_seen_at)}</td>
          <td class="py-3 px-3 sm:px-4 text-right">
            <div class="flex items-center justify-end gap-1 sm:gap-1.5">
              <button onclick="openDesktop('${d.id}', '${escapeQuotes(d.hostname)}')" ${!isOnline ? 'disabled' : ''} class="p-2 sm:px-2.5 sm:py-1.5 rounded-lg bg-emerald-600/10 text-emerald-400 border border-emerald-500/20 hover:bg-emerald-600/20 disabled:opacity-40 disabled:cursor-not-allowed text-xs font-semibold inline-flex items-center gap-1.5 transition-all" title="Remote Desktop">
                <i data-lucide="layout" class="w-3.5 h-3.5"></i> <span class="hidden md:inline">Desktop</span>
              </button>
              <button onclick="openFilesModal('${d.id}', '${escapeQuotes(d.hostname)}')" ${!isOnline ? 'disabled' : ''} class="p-2 sm:px-2.5 sm:py-1.5 rounded-lg bg-amber-500/10 text-amber-400 border border-amber-500/20 hover:bg-amber-500/20 disabled:opacity-40 disabled:cursor-not-allowed text-xs font-semibold inline-flex items-center gap-1.5 transition-all" title="File Explorer">
                <i data-lucide="folder" class="w-3.5 h-3.5"></i> <span class="hidden md:inline">Files</span>
              </button>
              <button onclick="openTerminal('${d.id}', '${escapeQuotes(d.hostname)}')" ${!isOnline ? 'disabled' : ''} class="p-2 sm:px-2.5 sm:py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-xs font-semibold text-white inline-flex items-center gap-1.5 shadow-sm transition-all" title="Interactive Shell">
                <i data-lucide="terminal" class="w-3.5 h-3.5"></i> <span class="hidden md:inline">Shell</span>
              </button>
              <button onclick="deleteDevice('${d.id}', '${escapeQuotes(d.hostname)}')" title="Hapus perangkat dari GoMesh" class="p-2 rounded-lg bg-rose-500/10 text-rose-400 border border-rose-500/20 hover:bg-rose-500/25 hover:text-rose-300 text-xs font-semibold inline-flex items-center justify-center transition-all ml-0.5 sm:ml-1">
                <i data-lucide="trash-2" class="w-3.5 h-3.5"></i>
              </button>
            </div>
          </td>
        </tr>
      `;
    }).join('');
  }

  if (window.lucide) lucide.createIcons();
}

async function deleteDevice(id, hostname) {
  const confirmed = await showConfirmModal({
    title: 'Hapus Perangkat',
    message: `Apakah Anda yakin ingin menghapus perangkat "${hostname}" (${id.substring(0, 8)}) dari daftar GoMesh? Riwayat audit sesi perangkat ini juga akan dihapus.`,
    confirmText: 'Hapus Sekarang',
    cancelText: 'Batal',
    isDanger: true,
  });
  if (!confirmed) return;

  try {
    const res = await fetch(`/api/devices/${id}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Gagal menghapus perangkat');
    }
    showToast(`Perangkat ${hostname} berhasil dihapus`, 'success');
    fetchDevices();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// --- INITIALIZE & EVENT LISTENERS ---
async function initDashboard() {
  try {
    const res = await fetch('/api/auth/me');
    if (res.status === 401) {
      window.location.href = '/login';
      return;
    }
    const data = await res.json();
    if (data.authenticated && data.user) {
      currentUser = data.user;
      const navUser = document.getElementById('nav-username');
      const navRole = document.getElementById('nav-role');
      if (navUser) navUser.innerText = currentUser.username;
      if (navRole) navRole.innerText = currentUser.role;
    }
  } catch (e) {
    console.error('Auth verification error:', e);
  }

  fetchDevices();
  loadActiveToken();
  setInterval(fetchDevices, 3000);
}

// Modal closing and shortcut management
window.addEventListener('keydown', (e) => {
  const deskModal = document.getElementById('desktop-modal');
  if (deskModal && !deskModal.classList.contains('hidden')) {
    if (e.key === 'Escape' && !desktopControlEnabled) {
      if (!document.fullscreenElement) closeDesktopModal();
      return;
    }
    if (desktopControlEnabled && activeDesktopWS && activeDesktopWS.readyState === WebSocket.OPEN) {
      if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA')) return;
      activeDesktopWS.send(JSON.stringify({ event: 'keydown', key: e.key }));
      if (['Tab', 'Backspace', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', ' '].includes(e.key)) {
        e.preventDefault();
      }
      return;
    }
  }

  if (e.key === 'Escape') {
    const confirmModal = document.getElementById('custom-confirm-modal');
    if (confirmModal && !confirmModal.classList.contains('hidden')) {
      document.getElementById('confirm-modal-cancel-btn')?.click();
      return;
    }
    const promptModal = document.getElementById('custom-prompt-modal');
    if (promptModal && !promptModal.classList.contains('hidden')) {
      document.getElementById('prompt-modal-cancel-btn')?.click();
      return;
    }
    const modal2FA = document.getElementById('modal-2fa');
    if (modal2FA && !modal2FA.classList.contains('hidden')) close2FAModal();
    const changePwdModal = document.getElementById('modal-change-password');
    if (changePwdModal && !changePwdModal.classList.contains('hidden')) closeChangePasswordModal();
    const termModal = document.getElementById('terminal-modal');
    if (termModal && !termModal.classList.contains('hidden')) closeTerminal();
    const filesModal = document.getElementById('files-modal');
    if (filesModal && !filesModal.classList.contains('hidden')) closeFilesModal();
    if (deskModal && !deskModal.classList.contains('hidden') && !document.fullscreenElement) closeDesktopModal();
    const installModal = document.getElementById('install-modal');
    if (installModal && !installModal.classList.contains('hidden')) closeInstallModal();
    const auditsModal = document.getElementById('audits-modal');
    if (auditsModal && !auditsModal.classList.contains('hidden')) closeAuditsModal();
  }
});

window.addEventListener('keyup', (e) => {
  const deskModal = document.getElementById('desktop-modal');
  if (deskModal && !deskModal.classList.contains('hidden')) {
    if (desktopControlEnabled && activeDesktopWS && activeDesktopWS.readyState === WebSocket.OPEN) {
      if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA')) return;
      activeDesktopWS.send(JSON.stringify({ event: 'keyup', key: e.key }));
    }
  }
});

// Start dashboard on load
document.addEventListener('DOMContentLoaded', () => {
  initDashboard();
});
