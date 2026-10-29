// auth.js - User authentication settings, 2FA, change password, and logout

let currentSetupSecret = '';

async function handleLogout() {
  const confirmed = await showConfirmModal({
    title: 'Konfirmasi Logout',
    message: 'Apakah Anda yakin ingin keluar dari konsol GoMesh?',
    confirmText: 'Ya, Logout',
    isDanger: true
  });
  if (!confirmed) return;

  try {
    await fetch('/api/auth/logout', { method: 'POST' });
  } catch (e) {
    console.error('Logout error:', e);
  }
  showToast('Logout berhasil', 'info');
  window.location.href = '/login';
}

// --- 2FA SECURITY SETTINGS ---
async function open2FAModal() {
  document.getElementById('modal-2fa').classList.remove('hidden');
  if (window.lucide) lucide.createIcons();
  await refresh2FAStatus();
}

function close2FAModal() {
  document.getElementById('modal-2fa').classList.add('hidden');
}

async function refresh2FAStatus() {
  const banner = document.getElementById('2fa-status-banner');
  const icon = document.getElementById('2fa-status-icon');
  const text = document.getElementById('2fa-status-text');
  const badge = document.getElementById('2fa-status-badge');
  const setupSec = document.getElementById('2fa-setup-section');
  const disableSec = document.getElementById('2fa-disable-section');

  try {
    const res = await fetch('/api/auth/2fa/status');
    const data = await res.json();
    if (data.enabled) {
      banner.className = 'p-3 rounded-xl border border-emerald-500/20 bg-emerald-500/10 flex items-center justify-between text-emerald-400';
      icon.className = 'w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse';
      text.innerText = 'Two-Factor Authentication Aktif';
      badge.className = 'text-[10px] uppercase font-bold px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/30';
      badge.innerText = 'Active';

      setupSec.classList.add('hidden');
      disableSec.classList.remove('hidden');
    } else {
      banner.className = 'p-3 rounded-xl border border-amber-500/20 bg-amber-500/10 flex items-center justify-between text-amber-400';
      icon.className = 'w-2.5 h-2.5 rounded-full bg-amber-400';
      text.innerText = 'Two-Factor Authentication Belum Aktif';
      badge.className = 'text-[10px] uppercase font-bold px-2 py-0.5 rounded bg-amber-500/20 text-amber-300 border border-amber-500/30';
      badge.innerText = 'Disabled';

      disableSec.classList.add('hidden');
      setupSec.classList.remove('hidden');
      load2FASetup();
    }
  } catch (err) {
    showToast('Gagal memuat status 2FA: ' + err.message, 'error');
  }
}

async function load2FASetup() {
  const qrImg = document.getElementById('2fa-qr-img');
  const qrLoading = document.getElementById('2fa-qr-loading');
  if (qrImg) qrImg.classList.add('hidden');
  if (qrLoading) qrLoading.classList.remove('hidden');

  try {
    const res = await fetch('/api/auth/2fa/setup', { method: 'POST' });
    const data = await res.json();
    if (res.ok && data.secret) {
      currentSetupSecret = data.secret;
      document.getElementById('2fa-secret-text').value = data.secret;
      if (data.qr_url && qrImg) {
        qrImg.src = data.qr_url;
      }
    }
  } catch (err) {
    showToast('Gagal memuat konfigurasi 2FA', 'error');
  }
}

function copy2FASecret() {
  const secret = document.getElementById('2fa-secret-text').value;
  if (!secret) return;
  navigator.clipboard.writeText(secret);
  showToast('Secret key 2FA disalin ke clipboard!', 'success');
}

async function handleEnable2FA(e) {
  e.preventDefault();
  const code = document.getElementById('2fa-confirm-code').value.trim();
  const btn = document.getElementById('2fa-enable-btn');
  if (!code || code.length !== 6) {
    showToast('Masukkan 6 digit kode dari authenticator', 'warning');
    return;
  }

  btn.disabled = true;
  btn.innerText = 'Memverifikasi...';

  try {
    const res = await fetch('/api/auth/2fa/enable', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ secret: currentSetupSecret, code: code })
    });
    const data = await res.json();
    if (res.ok && data.success) {
      showToast('2FA berhasil diaktifkan untuk akun Anda!', 'success');
      document.getElementById('2fa-confirm-code').value = '';
      refresh2FAStatus();
    } else {
      showToast(data.error || 'Kode verifikasi 2FA tidak valid', 'error');
    }
  } catch (err) {
    showToast('Gagal mengaktifkan 2FA: ' + err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerText = 'Aktifkan 2FA';
  }
}

async function handleDisable2FA(e) {
  e.preventDefault();
  const password = document.getElementById('2fa-disable-password').value;
  const btn = document.getElementById('2fa-disable-btn');
  if (!password) return;

  const confirmed = await showConfirmModal({
    title: 'Nonaktifkan 2FA',
    message: 'Apakah Anda yakin ingin menonaktifkan 2FA? Akun Anda tidak lagi dilindungi verifikasi dua langkah.',
    confirmText: 'Ya, Nonaktifkan',
    isDanger: true
  });
  if (!confirmed) return;

  btn.disabled = true;
  btn.innerText = 'Memproses...';

  try {
    const res = await fetch('/api/auth/2fa/disable', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: password })
    });
    const data = await res.json();
    if (res.ok && data.success) {
      showToast('2FA telah dinonaktifkan.', 'info');
      document.getElementById('2fa-disable-password').value = '';
      refresh2FAStatus();
    } else {
      showToast(data.error || 'Password salah', 'error');
    }
  } catch (err) {
    showToast('Gagal menonaktifkan 2FA: ' + err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerText = 'Nonaktifkan 2FA';
  }
}

// --- CHANGE PASSWORD ---
function openChangePasswordModal() {
  document.getElementById('change-pwd-current').value = '';
  document.getElementById('change-pwd-new').value = '';
  document.getElementById('change-pwd-confirm').value = '';
  document.getElementById('change-pwd-error').classList.add('hidden');
  document.getElementById('change-pwd-success').classList.add('hidden');
  document.getElementById('modal-change-password').classList.remove('hidden');
  if (window.lucide) lucide.createIcons();
  setTimeout(() => document.getElementById('change-pwd-current')?.focus(), 50);
}

function closeChangePasswordModal() {
  document.getElementById('modal-change-password').classList.add('hidden');
}

async function handleChangePassword(e) {
  e.preventDefault();
  const currentPassword = document.getElementById('change-pwd-current').value;
  const newPassword = document.getElementById('change-pwd-new').value;
  const confirmPassword = document.getElementById('change-pwd-confirm').value;
  const errorDiv = document.getElementById('change-pwd-error');
  const errorMsg = document.getElementById('change-pwd-error-msg');
  const successDiv = document.getElementById('change-pwd-success');
  const submitBtn = document.getElementById('change-pwd-submit-btn');

  errorDiv.classList.add('hidden');
  successDiv.classList.add('hidden');

  if (!currentPassword) {
    errorMsg.innerText = 'Kata sandi saat ini wajib diisi.';
    errorDiv.classList.remove('hidden');
    return;
  }

  if (newPassword.length < 6) {
    errorMsg.innerText = 'Kata sandi baru minimal 6 karakter.';
    errorDiv.classList.remove('hidden');
    return;
  }

  if (newPassword !== confirmPassword) {
    errorMsg.innerText = 'Konfirmasi kata sandi baru tidak cocok.';
    errorDiv.classList.remove('hidden');
    return;
  }

  submitBtn.disabled = true;
  submitBtn.innerHTML = '<i data-lucide="loader-2" class="w-4 h-4 animate-spin"></i> Menyimpan...';
  if (window.lucide) lucide.createIcons();

  try {
    const res = await fetch('/api/auth/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword
      })
    });
    const data = await res.json();
    if (res.ok && data.success) {
      successDiv.classList.remove('hidden');
      showToast('Kata sandi berhasil diperbarui!', 'success');
      document.getElementById('change-pwd-current').value = '';
      document.getElementById('change-pwd-new').value = '';
      document.getElementById('change-pwd-confirm').value = '';
      setTimeout(() => {
        closeChangePasswordModal();
      }, 1200);
    } else {
      errorMsg.innerText = data.error || 'Gagal mengubah kata sandi.';
      errorDiv.classList.remove('hidden');
      showToast(data.error || 'Gagal mengubah kata sandi', 'error');
    }
  } catch (err) {
    errorMsg.innerText = 'Terjadi kesalahan jaringan: ' + err.message;
    errorDiv.classList.remove('hidden');
    showToast('Kesalahan jaringan: ' + err.message, 'error');
  } finally {
    submitBtn.disabled = false;
    submitBtn.innerHTML = '<span>Simpan Perubahan</span>';
    if (window.lucide) lucide.createIcons();
  }
}
