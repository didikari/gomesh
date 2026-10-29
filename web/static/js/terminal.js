// terminal.js - Interactive Web Terminal (xterm.js + PTY WebSocket)

let activeTerm = null;
let activeFitAddon = null;
let activeWS = null;

function openTerminal(deviceID, hostname) {
  const modal = document.getElementById('terminal-modal');
  const title = document.getElementById('term-title');
  const container = document.getElementById('terminal-container');

  modal.classList.remove('hidden');
  title.innerText = `Terminal: ${hostname} (${deviceID.substring(0, 8)})`;
  container.innerHTML = '';

  activeTerm = new Terminal({
    theme: {
      background: '#0f172a',
      foreground: '#f8fafc',
      cursor: '#818cf8',
      selectionBackground: '#4338ca',
      black: '#1e293b',
      red: '#f43f5e',
      green: '#10b981',
      yellow: '#f59e0b',
      blue: '#3b82f6',
      magenta: '#d946ef',
      cyan: '#06b6d4',
      white: '#f1f5f9'
    },
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
    fontSize: 13,
    lineHeight: 1.25,
    cursorBlink: true,
  });

  activeFitAddon = new FitAddon.FitAddon();
  activeTerm.loadAddon(activeFitAddon);
  activeTerm.loadAddon(new WebLinksAddon.WebLinksAddon());

  activeTerm.open(container);
  activeFitAddon.fit();

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/terminal?device_id=${deviceID}`;

  activeWS = new WebSocket(wsUrl);

  activeWS.onopen = () => {
    activeFitAddon.fit();
    activeWS.send(JSON.stringify({
      type: 'resize',
      cols: activeTerm.cols,
      rows: activeTerm.rows
    }));
    const dimLabel = document.getElementById('term-dimension');
    if (dimLabel) dimLabel.innerText = `cols: ${activeTerm.cols}, rows: ${activeTerm.rows}`;
  };

  activeWS.onmessage = (event) => {
    activeTerm.write(event.data);
  };

  activeWS.onclose = () => {
    activeTerm.write('\r\n\x1b[31;1m[GoMesh Central]: Sesi terminal berakhir atau terputus.\x1b[0m\r\n');
  };

  activeWS.onerror = (err) => {
    activeTerm.write('\r\n\x1b[31;1m[GoMesh Central]: Gagal membuka koneksi WebSocket terminal.\x1b[0m\r\n');
  };

  activeTerm.onData((data) => {
    if (activeWS && activeWS.readyState === WebSocket.OPEN) {
      activeWS.send(data);
    }
  });

  activeTerm.focus();
}

function closeTerminal() {
  const modal = document.getElementById('terminal-modal');
  modal.classList.add('hidden');
  if (activeWS) {
    activeWS.close();
    activeWS = null;
  }
  if (activeTerm) {
    activeTerm.dispose();
    activeTerm = null;
  }
  activeFitAddon = null;
}

window.addEventListener('resize', () => {
  if (activeFitAddon && activeTerm) {
    activeFitAddon.fit();
    if (activeWS && activeWS.readyState === WebSocket.OPEN) {
      activeWS.send(JSON.stringify({
        type: 'resize',
        cols: activeTerm.cols,
        rows: activeTerm.rows
      }));
    }
  }
});
