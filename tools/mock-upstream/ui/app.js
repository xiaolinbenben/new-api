const loginPanel = document.getElementById('login-panel');
const loginForm = document.getElementById('login-form');
const loginError = document.getElementById('login-error');
const overviewCards = document.getElementById('overview-cards');
const actionStatus = document.getElementById('action-status');
const workerStatusBadge = document.getElementById('worker-status-badge');

let pollTimer = null;

function zhStatus(status) {
  const mapping = {
    running: '运行中',
    stopped: '已停止',
    starting: '启动中',
    unknown: '未知',
  };
  return mapping[status] || status || '未知';
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
    ...options,
  });

  if (response.status === 401) {
    showLogin();
    throw new Error('未授权，请重新登录');
  }

  let payload = null;
  const text = await response.text();
  if (text) {
    payload = JSON.parse(text);
  }
  if (!response.ok) {
    throw new Error(payload?.message || `HTTP ${response.status}`);
  }
  return payload;
}

function showLogin() {
  loginPanel.classList.add('visible');
}

function hideLogin() {
  loginPanel.classList.remove('visible');
  loginError.textContent = '';
}

function setStatus(message, isError = false) {
  actionStatus.textContent = message || '';
  actionStatus.className = isError ? 'error' : 'muted';
}

function setBadge(status) {
  workerStatusBadge.textContent = zhStatus(status);
  workerStatusBadge.className = `badge ${status === 'running' ? 'running' : 'stopped'}`;
}

function joinLines(values = []) {
  return values.join('\n');
}

function splitLines(value) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean);
}

function value(id) {
  return document.getElementById(id).value;
}

function checked(id) {
  return document.getElementById(id).checked;
}

function numberValue(id) {
  return Number(document.getElementById(id).value || 0);
}

function fillConfig(config) {
  document.getElementById('worker-port').value = config.worker.port;
  document.getElementById('worker-pprof-port').value = config.worker.pprof_port;
  document.getElementById('worker-require-auth').checked = config.worker.require_auth;
  document.getElementById('worker-enable-pprof').checked = config.worker.enable_pprof;
  document.getElementById('worker-models').value = joinLines(config.worker.models);

  document.getElementById('random-mode').value = config.random.mode;
  document.getElementById('random-seed').value = config.random.seed;
  document.getElementById('latency-min').value = config.random.latency_ms.min;
  document.getElementById('latency-max').value = config.random.latency_ms.max;
  document.getElementById('error-rate').value = config.random.error_rate;
  document.getElementById('rate-429').value = config.random.too_many_requests_rate;
  document.getElementById('rate-500').value = config.random.server_error_rate;
  document.getElementById('rate-timeout').value = config.random.timeout_rate;
  document.getElementById('tokens-min').value = config.random.default_token_length.min;
  document.getElementById('tokens-max').value = config.random.default_token_length.max;
  document.getElementById('chunks-min').value = config.random.default_stream_chunks.min;
  document.getElementById('chunks-max').value = config.random.default_stream_chunks.max;

  fillTextConfig('chat', config.chat);
  fillTextConfig('resp', config.responses);

  document.getElementById('image-sizes').value = joinLines(config.images.sizes);
  document.getElementById('image-watermarks').value = joinLines(config.images.watermark_texts);
  document.getElementById('image-palette').value = joinLines(config.images.background_palette);
  document.getElementById('image-url-rate').value = config.images.response_url_rate;
  document.getElementById('image-watermark-rate').value = config.images.watermark_rate;
  document.getElementById('image-count-min').value = config.images.image_count.min;
  document.getElementById('image-count-max').value = config.images.image_count.max;
  document.getElementById('image-bytes-min').value = config.images.image_bytes.min;
  document.getElementById('image-bytes-max').value = config.images.image_bytes.max;

  document.getElementById('video-duration-min').value = config.videos.durations_seconds.min;
  document.getElementById('video-duration-max').value = config.videos.durations_seconds.max;
  document.getElementById('video-resolutions').value = joinLines(config.videos.resolutions);
  document.getElementById('video-failure-rate').value = config.videos.failure_rate;
  document.getElementById('video-fps-min').value = config.videos.fps.min;
  document.getElementById('video-fps-max').value = config.videos.fps.max;
  document.getElementById('video-poll-min').value = config.videos.poll_interval_ms.min;
  document.getElementById('video-poll-max').value = config.videos.poll_interval_ms.max;
  document.getElementById('video-bytes-min').value = config.videos.video_bytes.min;
  document.getElementById('video-bytes-max').value = config.videos.video_bytes.max;
  document.getElementById('video-jitter-min').value = config.videos.progress_jitter.min;
  document.getElementById('video-jitter-max').value = config.videos.progress_jitter.max;
}

function fillTextConfig(prefix, config) {
  document.getElementById(`${prefix}-text-min`).value = config.text_tokens.min;
  document.getElementById(`${prefix}-text-max`).value = config.text_tokens.max;
  document.getElementById(`${prefix}-allow-stream`).checked = config.allow_stream;
  document.getElementById(`${prefix}-usage`).value = config.usage_probability;
  document.getElementById(`${prefix}-tool-prob`).value = config.tool_call_probability;
  document.getElementById(`${prefix}-mcp-prob`).value = config.mcp_tool_probability;
  document.getElementById(`${prefix}-tool-min`).value = config.tool_call_count.min;
  document.getElementById(`${prefix}-tool-max`).value = config.tool_call_count.max;
  document.getElementById(`${prefix}-args-min`).value = config.tool_arguments_bytes.min;
  document.getElementById(`${prefix}-args-max`).value = config.tool_arguments_bytes.max;
  document.getElementById(`${prefix}-final-prob`).value = config.final_answer_probability;
  document.getElementById(`${prefix}-fp-prob`).value = config.system_fingerprint_probability;
}

function gatherTextConfig(prefix) {
  return {
    text_tokens: { min: numberValue(`${prefix}-text-min`), max: numberValue(`${prefix}-text-max`) },
    allow_stream: checked(`${prefix}-allow-stream`),
    usage_probability: Number(value(`${prefix}-usage`)),
    tool_call_probability: Number(value(`${prefix}-tool-prob`)),
    tool_call_count: { min: numberValue(`${prefix}-tool-min`), max: numberValue(`${prefix}-tool-max`) },
    tool_arguments_bytes: { min: numberValue(`${prefix}-args-min`), max: numberValue(`${prefix}-args-max`) },
    mcp_tool_probability: Number(value(`${prefix}-mcp-prob`)),
    final_answer_probability: Number(value(`${prefix}-final-prob`)),
    system_fingerprint_probability: Number(value(`${prefix}-fp-prob`)),
  };
}

function gatherConfig() {
  return {
    worker: {
      port: numberValue('worker-port'),
      pprof_port: numberValue('worker-pprof-port'),
      require_auth: checked('worker-require-auth'),
      enable_pprof: checked('worker-enable-pprof'),
      models: splitLines(value('worker-models')),
    },
    random: {
      mode: value('random-mode'),
      seed: numberValue('random-seed'),
      latency_ms: { min: numberValue('latency-min'), max: numberValue('latency-max') },
      error_rate: Number(value('error-rate')),
      too_many_requests_rate: Number(value('rate-429')),
      server_error_rate: Number(value('rate-500')),
      timeout_rate: Number(value('rate-timeout')),
      default_token_length: { min: numberValue('tokens-min'), max: numberValue('tokens-max') },
      default_stream_chunks: { min: numberValue('chunks-min'), max: numberValue('chunks-max') },
    },
    chat: gatherTextConfig('chat'),
    responses: gatherTextConfig('resp'),
    images: {
      sizes: splitLines(value('image-sizes')),
      watermark_texts: splitLines(value('image-watermarks')),
      background_palette: splitLines(value('image-palette')),
      response_url_rate: Number(value('image-url-rate')),
      watermark_rate: Number(value('image-watermark-rate')),
      image_count: { min: numberValue('image-count-min'), max: numberValue('image-count-max') },
      image_bytes: { min: numberValue('image-bytes-min'), max: numberValue('image-bytes-max') },
    },
    videos: {
      durations_seconds: { min: Number(value('video-duration-min')), max: Number(value('video-duration-max')) },
      resolutions: splitLines(value('video-resolutions')),
      failure_rate: Number(value('video-failure-rate')),
      fps: { min: numberValue('video-fps-min'), max: numberValue('video-fps-max') },
      poll_interval_ms: { min: numberValue('video-poll-min'), max: numberValue('video-poll-max') },
      video_bytes: { min: numberValue('video-bytes-min'), max: numberValue('video-bytes-max') },
      progress_jitter: { min: numberValue('video-jitter-min'), max: numberValue('video-jitter-max') },
    },
  };
}

function renderOverview(runtime, summary) {
  setBadge(runtime.worker.status);
  const cards = [
    ['Worker 端口', runtime.config.worker.port],
    ['Worker PID', runtime.worker.pid || '-'],
    ['总请求数', summary?.total_requests ?? 0],
    ['QPS (1分钟)', Number(summary?.current_qps ?? 0).toFixed(2)],
    ['错误率', `${((summary?.error_rate ?? 0) * 100).toFixed(2)}%`],
    ['活跃请求', summary?.active_requests ?? 0],
    ['活跃 SSE', summary?.active_sse ?? 0],
    ['视频任务数', summary?.video_tasks ?? 0],
  ];

  overviewCards.innerHTML = cards
    .map(
      ([label, val]) => `
        <div class="card">
          <div class="card-label">${label}</div>
          <div class="card-value">${val}</div>
        </div>`,
    )
    .join('');
}

function renderRoutes(routes) {
  const tbody = document.querySelector('#routes-table tbody');
  tbody.innerHTML = routes.items
    .map(
      (item) => `
        <tr>
          <td>${item.route}</td>
          <td>${item.requests}</td>
          <td>${item.successes}</td>
          <td>${item.errors}</td>
          <td>${item.active}</td>
          <td>${item.average_ms.toFixed(2)}</td>
          <td>${item.p50_ms.toFixed(2)}</td>
          <td>${item.p95_ms.toFixed(2)}</td>
          <td>${item.last_status || '-'}</td>
        </tr>`,
    )
    .join('');
}

function renderSimpleTable(selector, rows, formatter) {
  const tbody = document.querySelector(`${selector} tbody`);
  tbody.innerHTML = rows
    .map((row) => `<tr><td>${formatter(row)}</td></tr>`)
    .join('');
}

function formatTime(value) {
  if (!value) return '-';
  return new Date(value).toLocaleTimeString();
}

async function loadRuntime() {
  const [runtime, config] = await Promise.all([api('/api/runtime'), api('/api/config')]);
  fillConfig(config);
  let summary = {
    total_requests: 0,
    current_qps: 0,
    error_rate: 0,
    active_requests: 0,
    active_sse: 0,
    video_tasks: 0,
  };
  let routes = { items: [] };
  let events = { errors: [], requests: [], videos: [] };

  if (runtime.worker.status === 'running') {
    [summary, routes, events] = await Promise.all([
      api('/api/stats/summary'),
      api('/api/stats/routes'),
      api('/api/stats/events'),
    ]);
  }

  renderOverview(runtime, summary);
  renderRoutes(routes);
  renderSimpleTable('#errors-table', events.errors.slice(0, 10), (row) =>
    `${formatTime(row.timestamp)} | ${row.route} | ${row.status_code} | ${row.error || '-'}`,
  );
  renderSimpleTable('#requests-table', events.requests.slice(0, 10), (row) =>
    `${formatTime(row.timestamp)} | ${row.method} ${row.route} | ${row.status_code} | ${row.latency_ms}ms`,
  );
  renderSimpleTable('#videos-table', events.videos.slice(0, 10), (row) =>
    `${formatTime(row.timestamp)} | ${row.task_id} | ${row.status} | ${row.resolution} @ ${row.fps}fps`,
  );
}

async function saveConfig() {
  const payload = gatherConfig();
    await api('/api/config', {
      method: 'PUT',
      body: JSON.stringify(payload),
    });
  setStatus('配置已保存');
  await loadRuntime();
}

async function workerAction(path, message) {
  await api(path, { method: 'POST', body: '{}' });
  setStatus(message);
  await loadRuntime();
}

function startPolling() {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = setInterval(() => {
    loadRuntime().catch((error) => setStatus(error.message, true));
  }, 2000);
}

loginForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  loginError.textContent = '';
  try {
    await api('/api/admin/login', {
      method: 'POST',
      body: JSON.stringify({ password: document.getElementById('login-password').value }),
    });
    hideLogin();
    await loadRuntime();
    startPolling();
  } catch (error) {
    loginError.textContent = error.message;
  }
});

document.getElementById('save-config').addEventListener('click', () => saveConfig().catch((error) => setStatus(error.message, true)));
document.getElementById('refresh-all').addEventListener('click', () => loadRuntime().catch((error) => setStatus(error.message, true)));
document.getElementById('worker-start').addEventListener('click', () => workerAction('/api/worker/start', 'Worker 已启动').catch((error) => setStatus(error.message, true)));
document.getElementById('worker-stop').addEventListener('click', () => workerAction('/api/worker/stop', 'Worker 已停止').catch((error) => setStatus(error.message, true)));
document.getElementById('worker-restart').addEventListener('click', () => workerAction('/api/worker/restart', 'Worker 已重启').catch((error) => setStatus(error.message, true)));
document.getElementById('reset-stats').addEventListener('click', async () => {
  try {
    await api('/api/stats/reset', { method: 'POST', body: '{}' });
    setStatus('统计已重置');
    await loadRuntime();
  } catch (error) {
    setStatus(error.message, true);
  }
});

document.getElementById('logout').addEventListener('click', async () => {
  try {
    await api('/api/admin/logout', { method: 'POST', body: '{}' });
  } catch (_) {
    // ignore
  }
  showLogin();
});

showLogin();
