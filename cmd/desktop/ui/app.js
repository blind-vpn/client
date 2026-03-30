const $ = s => document.querySelector(s);

let state = {
  connected: false,
  connecting: false,
  selectedServer: null,
  servers: [],
  registration: null,
  expired: true,
};

function showLogin() {
  $('#view-login').style.display = '';
  $('#view-main').style.display = 'none';
}

function showMain() {
  $('#view-login').style.display = 'none';
  $('#view-main').style.display = '';
  loadServers();
  loadAccountInfo();
  loadSettings();
}

async function api(path, opts = {}) {
  const resp = await fetch('/api' + path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  });
  const data = await resp.json();
  if (data.error) throw new Error(data.error);
  return data;
}

async function init() {
  try {
    const s = await api('/status');
    if (s.logged_in) {
      $('#account-display').textContent = s.account_id;
      state.connected = s.connected;
      showMain();
      updateConnectUI();
    } else {
      showLogin();
    }
  } catch {
    showLogin();
  }
}

// Login
$('#btn-create').onclick = async () => {
  $('#btn-create').disabled = true;
  $('#btn-create').textContent = 'Generating...';
  $('#login-error').textContent = '';
  try {
    const acct = await api('/account/create', { method: 'POST' });
    $('#account-display').textContent = acct.account_id;
    showMain();
  } catch (e) {
    $('#login-error').textContent = e.message;
  } finally {
    $('#btn-create').disabled = false;
    $('#btn-create').textContent = 'Generate Account';
  }
};

$('#btn-login').onclick = async () => {
  const id = $('#input-login').value.trim();
  if (id.length !== 16) {
    $('#login-error').textContent = 'Account number must be 16 digits';
    return;
  }
  $('#login-error').textContent = '';
  try {
    await api('/account/info?id=' + id);
    $('#account-display').textContent = id;
    showMain();
  } catch (e) {
    $('#login-error').textContent = e.message;
  }
};

$('#input-login').onkeydown = e => { if (e.key === 'Enter') $('#btn-login').click(); };

$('#btn-logout').onclick = () => {
  if (state.connected) return;
  showLogin();
};

// Account info + time banner
async function loadAccountInfo() {
  try {
    const info = await api('/account/info');
    const expiresAt = new Date(info.expires_at);
    const now = new Date();
    const banner = $('#time-banner');
    const text = $('#time-text');

    const msRemaining = expiresAt - now;
    if (msRemaining < 60000) { // Less than 1 minute = expired
      state.expired = true;
      banner.style.display = 'flex';
      banner.className = 'time-banner';
      text.textContent = 'No time remaining';
    } else {
      state.expired = false;
      const days = Math.floor(msRemaining / (1000 * 60 * 60 * 24));
      const hours = Math.floor((msRemaining % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
      banner.style.display = 'flex';
      banner.className = 'time-banner active';
      if (days > 0) {
        text.textContent = days + ' day' + (days !== 1 ? 's' : '') + ' remaining';
      } else {
        text.textContent = hours + ' hour' + (hours !== 1 ? 's' : '') + ' remaining';
      }
    }
    updateConnectUI();
  } catch (e) {
    console.error('account info:', e);
  }
}

// Servers
async function loadServers() {
  try {
    const data = await api('/servers');
    state.servers = data.servers || [];
    renderServers();
  } catch (e) {
    console.error('load servers:', e);
  }
}

function countryFlag(code) {
  if (!code || code.length !== 2) return '';
  return String.fromCodePoint(...[...code.toUpperCase()].map(c => c.charCodeAt(0) + 0x1F1A5));
}

function renderServers() {
  const list = $('#server-list');
  list.innerHTML = '<div class="section-label">LOCATION</div>';
  if (state.servers.length === 0) {
    list.innerHTML += '<div style="color:var(--muted);font-size:13px;padding:8px 0;">No servers available</div>';
    return;
  }

  const sel = document.createElement('select');
  sel.className = 'server-select';
  const placeholder = document.createElement('option');
  placeholder.textContent = 'Select a location...';
  placeholder.value = '';
  placeholder.disabled = true;
  placeholder.selected = !state.selectedServer;
  sel.appendChild(placeholder);

  state.servers.forEach((s, i) => {
    const opt = document.createElement('option');
    opt.value = i;
    opt.textContent = countryFlag(s.region.country) + '  ' + s.region.city + ', ' + s.region.country;
    if (state.selectedServer?.id === s.id) opt.selected = true;
    sel.appendChild(opt);
  });

  sel.onchange = () => {
    const idx = parseInt(sel.value);
    if (!isNaN(idx)) selectServer(state.servers[idx]);
  };

  list.appendChild(sel);
}

function selectServer(s) {
  state.selectedServer = s;
  renderServers();
}

// Connect / Disconnect
$('#btn-connect').onclick = async () => {
  if (state.connecting) return;

  if (state.expired) {
    window.open('https://blind-vpn.com/account', '_blank');
    return;
  }

  if (state.connected) {
    state.connecting = true;
    updateConnectUI();
    try {
      await api('/disconnect', { method: 'POST' });
      state.connected = false;
      state.registration = null;
      fetch('/api/tray/disconnected').catch(() => {});
    } catch (e) {
      alert('Disconnect failed: ' + e.message);
    }
    state.connecting = false;
    updateConnectUI();
    return;
  }

  if (!state.selectedServer) {
    alert('Select a server first');
    return;
  }

  state.connecting = true;
  updateConnectUI();

  try {
    const reg = await api('/keys/register', { method: 'POST' });

    let connInfo;
    if (reg.server_id) {
      connInfo = reg;
    } else if (reg.keys && reg.keys.length > 0) {
      const key = reg.keys[0];
      connInfo = {
        server_id: key.server_id,
        server_ip: state.selectedServer.public_ip,
        server_pubkey: state.selectedServer.wg_pubkey,
        server_port: state.selectedServer.wg_port,
        allowed_ip: key.allowed_ip,
      };
    }

    if (!connInfo.server_ip) connInfo.server_ip = state.selectedServer.public_ip;
    if (!connInfo.server_pubkey) connInfo.server_pubkey = state.selectedServer.wg_pubkey;
    if (!connInfo.server_port) connInfo.server_port = state.selectedServer.wg_port;

    await api('/connect', {
      method: 'POST',
      body: JSON.stringify(connInfo),
    });

    state.connected = true;
    state.registration = connInfo;
    fetch('/api/tray/connected').catch(() => {});
  } catch (e) {
    alert('Connection failed:\n\n' + e.message);
    state.connected = false;
  }

  state.connecting = false;
  updateConnectUI();
};

function updateConnectUI() {
  const btn = $('#btn-connect');
  const text = $('#status-text');
  const server = $('#status-server');

  btn.className = 'connect-btn';
  btn.disabled = false;

  if (state.expired && !state.connected) {
    btn.classList.add('disconnected');
    btn.style.opacity = '0.5';
    text.textContent = 'Add time to connect';
    text.className = 'status-text';
    server.textContent = '';
    return;
  }

  btn.style.opacity = '';

  if (state.connecting) {
    btn.classList.add('connecting');
    text.textContent = state.connected ? 'Disconnecting...' : 'Connecting...';
    text.className = 'status-text';
  } else if (state.connected) {
    btn.classList.add('connected');
    text.textContent = 'Connected';
    text.className = 'status-text on';
    if (state.selectedServer) {
      server.textContent = countryFlag(state.selectedServer.region.country) + ' ' + state.selectedServer.region.city;
    }
  } else {
    btn.classList.add('disconnected');
    text.textContent = 'Not Connected';
    text.className = 'status-text';
    server.textContent = '';
  }
}

// Settings
async function loadSettings() {
  try {
    const s = await api('/settings');
    $('#chk-autostart').checked = s.autostart;
    $('#chk-autoconnect').checked = s.autoconnect;
    $('#chk-killswitch').checked = s.kill_switch;
    $('#chk-padding').checked = s.padding;
  } catch {}
}

function bindToggle(id, key) {
  $(id).onchange = async () => {
    const val = $(id).checked;
    try {
      await api('/settings/update', {
        method: 'POST',
        body: JSON.stringify({ [key]: val }),
      });
    } catch (e) {
      $(id).checked = !val;
    }
  };
}

bindToggle('#chk-autostart', 'autostart');
bindToggle('#chk-autoconnect', 'autoconnect');
bindToggle('#chk-killswitch', 'kill_switch');
bindToggle('#chk-padding', 'padding');

init();
