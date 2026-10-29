// utils.js - Global utilities, dialogs, toasts, and formatters

function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function showToast(message, type = 'info', duration = 3500) {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = 'pointer-events-auto flex items-start gap-3 p-3.5 rounded-xl shadow-2xl backdrop-blur-md border text-xs transition-all duration-300 transform translate-y-2 opacity-0 select-none';

  let borderBg = 'bg-slate-900/95 border-slate-800 text-slate-200';
  let iconMarkup = '<i data-lucide="info" class="w-4 h-4 text-indigo-400 shrink-0 mt-0.5"></i>';

  if (type === 'success') {
    borderBg = 'bg-slate-900/95 border-emerald-500/40 text-emerald-200 shadow-emerald-500/10';
    iconMarkup = '<i data-lucide="check-circle" class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5"></i>';
  } else if (type === 'error') {
    borderBg = 'bg-slate-900/95 border-rose-500/40 text-rose-200 shadow-rose-500/10';
    iconMarkup = '<i data-lucide="alert-circle" class="w-4 h-4 text-rose-400 shrink-0 mt-0.5"></i>';
  } else if (type === 'warning') {
    borderBg = 'bg-slate-900/95 border-amber-500/40 text-amber-200 shadow-amber-500/10';
    iconMarkup = '<i data-lucide="alert-triangle" class="w-4 h-4 text-amber-400 shrink-0 mt-0.5"></i>';
  }

  toast.className += ' ' + borderBg;
  toast.innerHTML = `
    ${iconMarkup}
    <div class="flex-1 leading-snug font-medium">${escapeHtml(message)}</div>
    <button type="button" class="text-slate-400 hover:text-slate-200 shrink-0 p-0.5" onclick="this.parentElement.remove()">
      <i data-lucide="x" class="w-3.5 h-3.5"></i>
    </button>
  `;

  container.appendChild(toast);
  if (window.lucide) lucide.createIcons();

  requestAnimationFrame(() => {
    toast.classList.remove('translate-y-2', 'opacity-0');
    toast.classList.add('translate-y-0', 'opacity-100');
  });

  setTimeout(() => {
    toast.classList.add('opacity-0', 'translate-x-4');
    setTimeout(() => toast.remove(), 300);
  }, duration);
}

// Modern replacement for window.confirm()
function showConfirmModal({ title = 'Konfirmasi', message = 'Apakah Anda yakin?', confirmText = 'Lanjutkan', cancelText = 'Batal', isDanger = false } = {}) {
  return new Promise((resolve) => {
    const modal = document.getElementById('custom-confirm-modal');
    const titleEl = document.getElementById('confirm-modal-title');
    const msgEl = document.getElementById('confirm-modal-message');
    const confirmBtn = document.getElementById('confirm-modal-confirm-btn');
    const cancelBtn = document.getElementById('confirm-modal-cancel-btn');
    const iconContainer = document.getElementById('confirm-modal-icon-container');

    titleEl.innerText = title;
    msgEl.innerText = message;
    confirmBtn.innerText = confirmText;
    cancelBtn.innerText = cancelText;

    if (isDanger) {
      confirmBtn.className = 'px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-xs font-semibold text-white shadow-lg shadow-rose-600/20 transition-colors';
      iconContainer.className = 'p-2.5 rounded-xl bg-rose-500/10 text-rose-400 border border-rose-500/20 shrink-0';
    } else {
      confirmBtn.className = 'px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs font-semibold text-white shadow-lg shadow-indigo-600/20 transition-colors';
      iconContainer.className = 'p-2.5 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 shrink-0';
    }

    const cleanup = () => {
      modal.classList.add('hidden');
      confirmBtn.onclick = null;
      cancelBtn.onclick = null;
    };

    confirmBtn.onclick = () => { cleanup(); resolve(true); };
    cancelBtn.onclick = () => { cleanup(); resolve(false); };

    modal.classList.remove('hidden');
    confirmBtn.focus();
    if (window.lucide) lucide.createIcons();
  });
}

// Modern replacement for window.prompt()
function showPromptModal({ title = 'Input Diperlukan', message = 'Masukkan nilai:', placeholder = '', defaultValue = '', submitText = 'Simpan', cancelText = 'Batal' } = {}) {
  return new Promise((resolve) => {
    const modal = document.getElementById('custom-prompt-modal');
    const form = document.getElementById('custom-prompt-form');
    const titleEl = document.getElementById('prompt-modal-title');
    const msgEl = document.getElementById('prompt-modal-message');
    const input = document.getElementById('prompt-modal-input');
    const submitBtn = document.getElementById('prompt-modal-submit-btn');
    const cancelBtn = document.getElementById('prompt-modal-cancel-btn');

    titleEl.innerText = title;
    msgEl.innerText = message;
    input.placeholder = placeholder;
    input.value = defaultValue;
    submitBtn.innerText = submitText;
    cancelBtn.innerText = cancelText;

    const cleanup = () => {
      modal.classList.add('hidden');
      form.onsubmit = null;
      cancelBtn.onclick = null;
    };

    form.onsubmit = (e) => {
      e.preventDefault();
      const val = input.value;
      cleanup();
      resolve(val);
    };

    cancelBtn.onclick = () => {
      cleanup();
      resolve(null);
    };

    modal.classList.remove('hidden');
    input.focus();
    input.select();
    if (window.lucide) lucide.createIcons();
  });
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function timeAgo(dateString) {
  const date = new Date(dateString);
  const seconds = Math.floor((new Date() - date) / 1000);
  if (seconds < 5) return 'Just now';
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

window.alert = function(msg) {
  showToast(msg, 'info');
};
