// desktop.js - Remote Desktop Streaming & Interactive Input Injection

let activeDesktopWS = null;
let activeDesktopDeviceID = null;
let desktopControlEnabled = false;
let currentNativeWidth = 1920;
let currentNativeHeight = 1080;
let desktopLoadingTimer = null;

function openDesktop(deviceID, hostname) {
  activeDesktopDeviceID = deviceID;
  const modal = document.getElementById('desktop-modal');
  const title = document.getElementById('desktop-title');
  const metrics = document.getElementById('desktop-metrics');
  const loading = document.getElementById('desktop-loading');
  const canvas = document.getElementById('desktop-canvas');
  const ctx = canvas.getContext('2d');

  modal.classList.remove('hidden');
  title.innerText = `Remote Desktop: ${hostname} (${deviceID.substring(0, 8)})`;
  metrics.innerText = 'Menghubungkan stream...';
  loading.classList.remove('hidden');
  loading.innerHTML = `
    <i data-lucide="loader-2" class="w-8 h-8 animate-spin text-emerald-400"></i>
    <p class="text-sm font-medium">Memulai streaming layar remote...</p>
  `;
  if (window.lucide) lucide.createIcons();

  desktopControlEnabled = false;
  updateControlUI();

  if (desktopLoadingTimer) clearTimeout(desktopLoadingTimer);
  desktopLoadingTimer = setTimeout(() => {
    if (!loading.classList.contains('hidden')) {
      metrics.innerText = 'Menunggu display stream...';
      loading.innerHTML = `
        <div class="text-center p-4 max-w-sm space-y-2">
          <i data-lucide="alert-triangle" class="w-8 h-8 text-amber-400 mx-auto"></i>
          <p class="text-sm font-semibold text-white">Belum Ada Frame Diterima</p>
          <p class="text-xs text-slate-400 leading-relaxed">
            Jika komputer target adalah macOS, pastikan izin <strong>Screen Recording</strong> telah diaktifkan di <strong>System Settings > Privacy & Security > Screen Recording</strong>.
          </p>
        </div>
      `;
      if (window.lucide) lucide.createIcons();
    }
  }, 5000);

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/desktop?device_id=${deviceID}`;

  activeDesktopWS = new WebSocket(wsUrl);

  let frameCount = 0;
  let lastFpsCheck = Date.now();
  let currentFPS = 0;

  activeDesktopWS.onmessage = (event) => {
    try {
      const frame = JSON.parse(event.data);
      if (frame && frame.data) {
        clearTimeout(desktopLoadingTimer);
        loading.classList.add('hidden');

        const onDecoded = (drawable) => {
          if (canvas.width !== frame.width || canvas.height !== frame.height) {
            canvas.width = frame.width;
            canvas.height = frame.height;
            currentNativeWidth = frame.width;
            currentNativeHeight = frame.height;
          }
          ctx.drawImage(drawable, 0, 0);

          frameCount++;
          const now = Date.now();
          if (now - lastFpsCheck >= 1000) {
            currentFPS = frameCount;
            frameCount = 0;
            lastFpsCheck = now;
            metrics.innerText = `${frame.width}x${frame.height} • ${currentFPS} FPS • Sat-Set Ultra Low Latency`;
          }
        };

        if (window.createImageBitmap) {
          const byteCharacters = atob(frame.data);
          const byteNumbers = new Uint8Array(byteCharacters.length);
          for (let i = 0; i < byteCharacters.length; i++) {
            byteNumbers[i] = byteCharacters.charCodeAt(i);
          }
          const blob = new Blob([byteNumbers], { type: 'image/jpeg' });
          createImageBitmap(blob).then(bitmap => {
            onDecoded(bitmap);
            bitmap.close();
          }).catch(() => {
            const img = new Image();
            img.onload = () => onDecoded(img);
            img.src = `data:image/jpeg;base64,${frame.data}`;
          });
        } else {
          const img = new Image();
          img.onload = () => onDecoded(img);
          img.src = `data:image/jpeg;base64,${frame.data}`;
        }
      }
    } catch (e) {
      console.error('Frame decode error:', e);
    }
  };

  activeDesktopWS.onclose = () => {
    clearTimeout(desktopLoadingTimer);
    metrics.innerText = 'Stream disconnected';
    loading.classList.remove('hidden');
    loading.innerHTML = `<p class="text-rose-400 font-medium">Sesi Desktop Berakhir</p>`;
  };

  activeDesktopWS.onerror = () => {
    metrics.innerText = 'Stream error';
  };

  let lastMoveSent = 0;
  let lastSentX = -1;
  let lastSentY = -1;

  function sendInput(eventData) {
    if (!desktopControlEnabled || !activeDesktopWS || activeDesktopWS.readyState !== WebSocket.OPEN) return;
    activeDesktopWS.send(JSON.stringify(eventData));
  }

  function getCanvasCoordinates(e) {
    const rect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    return {
      x: Math.round((e.clientX - rect.left) * scaleX),
      y: Math.round((e.clientY - rect.top) * scaleY)
    };
  }

  canvas.onmousemove = (e) => {
    if (!desktopControlEnabled) return;
    const now = Date.now();
    if (now - lastMoveSent > 50) {
      const pos = getCanvasCoordinates(e);
      if (Math.abs(pos.x - lastSentX) >= 3 || Math.abs(pos.y - lastSentY) >= 3) {
        lastMoveSent = now;
        lastSentX = pos.x;
        lastSentY = pos.y;
        sendInput({ event: 'mousemove', x: pos.x, y: pos.y });
      }
    }
  };

  canvas.onmousedown = (e) => {
    if (!desktopControlEnabled) return;
    e.preventDefault();
    const pos = getCanvasCoordinates(e);
    lastSentX = pos.x;
    lastSentY = pos.y;
    sendInput({ event: 'mousedown', x: pos.x, y: pos.y, button: e.button });
    canvas.focus();
  };

  canvas.onmouseup = (e) => {
    if (!desktopControlEnabled) return;
    e.preventDefault();
    const pos = getCanvasCoordinates(e);
    sendInput({ event: 'mouseup', x: pos.x, y: pos.y, button: e.button });
  };

  canvas.onclick = (e) => {
    if (!desktopControlEnabled) {
      showToast("Mode 'View Only' aktif. Klik tombol 'Control: View Only' di pojok kanan atas untuk mengaktifkan kendali mouse & keyboard.", "info");
      return;
    }
    e.preventDefault();
  };

  canvas.onwheel = (e) => {
    if (!desktopControlEnabled) return;
    e.preventDefault();
    const pos = getCanvasCoordinates(e);
    sendInput({ event: 'wheel', x: pos.x, y: pos.y, delta_y: e.deltaY });
  };

  canvas.onkeydown = (e) => {
    if (!desktopControlEnabled) return;
    e.preventDefault();
    sendInput({ event: 'keydown', key: e.key });
  };

  canvas.onkeyup = (e) => {
    if (!desktopControlEnabled) return;
    e.preventDefault();
    sendInput({ event: 'keyup', key: e.key });
  };
}

function toggleDesktopControl() {
  desktopControlEnabled = !desktopControlEnabled;
  updateControlUI();
}

function updateControlUI() {
  const btn = document.getElementById('btn-control-mode');
  const label = document.getElementById('control-mode-label');
  const labelMobile = document.getElementById('control-mode-label-mobile');
  const canvas = document.getElementById('desktop-canvas');

  if (desktopControlEnabled) {
    btn.classList.remove('bg-slate-800', 'text-slate-200');
    btn.classList.add('bg-emerald-600', 'text-white');
    if (label) label.innerText = 'Control: Active (Interacting)';
    if (labelMobile) labelMobile.innerText = 'Active';
    canvas.classList.remove('cursor-crosshair');
    canvas.classList.add('cursor-default');
    canvas.focus();
  } else {
    btn.classList.remove('bg-emerald-600', 'text-white');
    btn.classList.add('bg-slate-800', 'text-slate-200');
    if (label) label.innerText = 'Control: View Only';
    if (labelMobile) labelMobile.innerText = 'View';
    canvas.classList.remove('cursor-crosshair');
    canvas.classList.add('cursor-default');
  }
}

function toggleDesktopFullscreen() {
  const container = document.getElementById('desktop-container');
  if (!document.fullscreenElement) {
    container.requestFullscreen().catch(err => showToast(err.message, 'error'));
  } else {
    document.exitFullscreen();
  }
}

function closeDesktopModal() {
  if (desktopLoadingTimer) {
    clearTimeout(desktopLoadingTimer);
    desktopLoadingTimer = null;
  }
  const modal = document.getElementById('desktop-modal');
  modal.classList.add('hidden');
  if (activeDesktopWS) {
    activeDesktopWS.close();
    activeDesktopWS = null;
  }
  activeDesktopDeviceID = null;
}
