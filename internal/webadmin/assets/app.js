const $ = (id) => document.getElementById(id);
let snapshot = null;
let credentialsLoaded = false;
let toastTimer = null;

async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: 'same-origin',
    ...options,
    headers: { 'Content-Type': 'application/json', 'X-AgentDock-Admin': '1', ...(options.headers || {}) }
  });
  let payload = {};
  try { payload = await response.json(); } catch (_) {}
  if (!response.ok) throw new Error(payload.message || `请求失败 (${response.status})`);
  return payload;
}

function toast(message, error = false) {
  const element = $('toast');
  element.textContent = message;
  element.className = `toast show${error ? ' error' : ''}`;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => element.className = 'toast', 4200);
}

function busy(active, text = '正在应用，请稍候…') {
  $('busy').hidden = !active;
  $('busy').querySelector('span').textContent = text;
}

function value(id, fallback = '') { return ($(id)?.value ?? fallback).trim(); }

async function refresh() {
  try {
    snapshot = await api('/api/snapshot', { headers: {} });
    render(snapshot);
    $('last-refresh').textContent = `上次刷新：${new Date().toLocaleTimeString()}`;
  } catch (error) {
    $('live-state').className = 'live-state offline';
    $('live-state').querySelector('span').textContent = '状态读取失败';
    toast(error.message, true);
  }
}

function render(data) {
  const service = data.service || {};
  const tunnel = data.tunnel || {};
  const runtime = data.runtime || {};
  const settings = runtime.settings || {};
  $('live-state').className = `live-state ${service.running && service.healthy ? 'online' : 'offline'}`;
  $('live-state').querySelector('span').textContent = service.running && service.healthy ? '运行正常' : (service.running ? '服务异常' : '服务已停止');
  $('service-state').textContent = service.running ? '正在运行' : '已停止';
  $('health-state').textContent = service.healthy ? '正常' : '不可用';
  $('tunnel-state').textContent = tunnel.mode === 'named' ? (tunnel.ready ? '固定域名已就绪' : '固定域名未就绪') : tunnel.mode === 'quick' ? '临时地址' : '未启用';
  $('version-state').textContent = data.version || 'dev';
  $('local-url').textContent = runtime.local_mcp_url || `http://127.0.0.1:${settings.port || 8765}/mcp`;
  $('public-url').textContent = runtime.public_mcp_url || '尚未配置';
  $('nexus-state').textContent = service.nexus_connected ? '已连接' : '尚未连接';
  document.querySelector(`input[name="tunnel-mode"][value="${tunnel.mode || 'none'}"]`).checked = true;
  $('server-url').value = runtime.named_server_url || '';
  $('token-state').textContent = runtime.tunnel_token_saved ? '已加密保存；留空不会覆盖' : '尚未保存 Token';
  $('port').value = settings.port || 8765;
  $('log-level').value = settings.log_level || 'info';
  $('mcp-apps-mode').value = settings.mcp_apps_mode || 'full';
  $('browser-cdp-url').value = settings.browser_cdp_url || '';
  $('browser-mode').value = !settings.browser_enabled ? 'off' : settings.browser_cdp_url ? 'cdp' : settings.browser_reuse_existing_cdp ? 'reuse' : 'isolated';
  updateModeCards(); updateBrowserField();
}

function updateModeCards() {
  const selected = document.querySelector('input[name="tunnel-mode"]:checked')?.value || 'none';
  document.querySelectorAll('.mode-card').forEach(card => card.classList.toggle('selected', card.querySelector('input').value === selected));
  $('fixed-config').classList.toggle('disabled', selected !== 'named');
}

function updateBrowserField() { $('cdp-field').hidden = $('browser-mode').value !== 'cdp'; }

document.querySelectorAll('.tab').forEach(tab => tab.addEventListener('click', () => {
  document.querySelectorAll('.tab,.panel').forEach(item => item.classList.remove('active'));
  tab.classList.add('active'); $(`panel-${tab.dataset.tab}`).classList.add('active');
}));
document.querySelectorAll('input[name="tunnel-mode"]').forEach(input => input.addEventListener('change', updateModeCards));
$('browser-mode').addEventListener('change', updateBrowserField);
$('refresh').addEventListener('click', refresh);

document.querySelectorAll('[data-copy]').forEach(button => button.addEventListener('click', async () => {
  const text = $(button.dataset.copy).textContent;
  if (!text || text === '尚未配置') return toast('当前没有可复制的地址', true);
  await navigator.clipboard.writeText(text); toast('已复制到剪贴板');
}));

document.querySelectorAll('[data-reveal]').forEach(button => button.addEventListener('click', async () => {
  try {
    if (!credentialsLoaded) {
      const result = await api('/api/credentials', { headers: {} });
      $('bearer-token').value = result.credentials.bearer_token;
      $('oauth-password').value = result.credentials.oauth_password;
      credentialsLoaded = true;
    }
    const input = $(button.dataset.reveal);
    input.type = input.type === 'password' ? 'text' : 'password';
    button.textContent = input.type === 'password' ? '显示' : '隐藏';
  } catch (error) { toast(error.message, true); }
}));

document.querySelectorAll('[data-service]').forEach(button => button.addEventListener('click', async () => {
  try { busy(true, '正在执行服务操作…'); await api('/api/service', { method: 'POST', body: JSON.stringify({ action: button.dataset.service }) }); toast('服务操作已完成'); setTimeout(refresh, 900); }
  catch (error) { toast(error.message, true); } finally { busy(false); }
}));

$('apply-tunnel').addEventListener('click', async () => {
  const mode = document.querySelector('input[name="tunnel-mode"]:checked')?.value || 'none';
  try {
    busy(true, '正在配置并启动隧道…');
    await api('/api/tunnel', { method: 'POST', body: JSON.stringify({ mode, server_url: value('server-url'), token: value('tunnel-token') }) });
    $('tunnel-token').value = ''; toast('公网访问配置已应用'); setTimeout(refresh, 1200);
  } catch (error) { toast(error.message, true); } finally { busy(false); }
});

$('restart-tunnel').addEventListener('click', async () => {
  try { busy(true, '正在重新启动 Tunnel…'); await api('/api/tunnel/action', { method: 'POST', body: JSON.stringify({ action: 'restart' }) }); toast('Tunnel 已重新启动'); setTimeout(refresh, 1000); }
  catch (error) { toast(error.message, true); } finally { busy(false); }
});

$('test-public').addEventListener('click', async () => {
  try { busy(true, '正在从本机测试公网地址…'); const result = await api('/api/public-test', { method: 'POST', body: '{}' }); toast(result.message); }
  catch (error) { toast(error.message, true); } finally { busy(false); }
});

$('save-config').addEventListener('click', async () => {
  try {
    busy(true, '正在保存设置并重启 Core…');
    await api('/api/config', { method: 'POST', body: JSON.stringify({
      port: Number($('port').value), log_level: value('log-level'), mcp_apps_mode: value('mcp-apps-mode'),
      browser_mode: value('browser-mode'), browser_cdp_url: value('browser-cdp-url')
    }) });
    toast('设置已保存'); setTimeout(refresh, 1200);
  } catch (error) { toast(error.message, true); } finally { busy(false); }
});

$('pair-nexus').addEventListener('click', async () => {
  if (!value('nexus-code')) return toast('请填写一次性配对码', true);
  try {
    busy(true, '正在配对 Nexus…');
    await api('/api/nexus/pair', { method: 'POST', body: JSON.stringify({ endpoint: value('nexus-endpoint'), code: value('nexus-code'), name: value('nexus-name') }) });
    $('nexus-code').value = ''; toast('Nexus 配对完成，请重新启动 Core');
  } catch (error) { toast(error.message, true); } finally { busy(false); }
});

$('check-update').addEventListener('click', async () => {
  try { busy(true, '正在检查更新…'); const result = await api('/api/update/check', { method: 'POST', body: '{}' }); toast(typeof result.result === 'string' ? result.result : JSON.stringify(result.result)); }
  catch (error) { toast(error.message, true); } finally { busy(false); }
});

$('run-update').addEventListener('click', async () => {
  if (!confirm('更新会替换 AgentDock 程序并重启相关服务。继续吗？')) return;
  try { busy(true, '正在下载并安装更新…'); const result = await api('/api/update/run', { method: 'POST', body: '{}' }); toast(result.message); setTimeout(refresh, 1200); }
  catch (error) { toast(error.message, true); } finally { busy(false); }
});

refresh();
