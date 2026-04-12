const { createApp } = Vue;

createApp({
  delimiters: ['[[', ']]'],
  data() {
    return {
      activeTab: 'overview',
      loading: false,
      loginVisible: true,
      loginPassword: '',
      loginLoading: false,
      savingConfig: false,
      runActionLoading: false,
      runtime: { worker: {} },
      summary: {},
      scenarioStats: { items: [] },
      samples: { requests: [], errors: [] },
      history: { items: [] },
      selectedSample: null,
      sampleDrawerVisible: false,
      selectedHistory: null,
      historyDrawerVisible: false,
      sampleFilter: '',
      pollTimer: null,
      config: {
        target: { base_url: '', headers: {}, insecure_skip_verify: false },
        run: { total_concurrency: 20, ramp_up_sec: 5, duration_sec: 30, max_requests: 0, request_timeout_ms: 30000 },
        sampling: { max_request_samples: 100, max_error_samples: 100, max_body_bytes: 4096, mask_headers: [] },
        scenarios: [],
      },
      targetHeadersText: '{}',
      maskHeadersText: '',
      scenarioDrawerVisible: false,
      editingScenarioIndex: -1,
      scenarioDraft: null,
    };
  },
  computed: {
    successRate() {
      const total = Number(this.summary.total_requests || 0);
      if (!total) return '0.00%';
      return `${(((this.summary.successes || 0) / total) * 100).toFixed(2)}%`;
    },
    statusCodeEntries() {
      return Object.entries(this.summary.status_codes || {})
        .map(([code, count]) => ({ code, count }))
        .sort((a, b) => Number(a.code) - Number(b.code));
    },
    filteredRequestSamples() {
      return this.filterSamples(this.samples.requests || []);
    },
    filteredErrorSamples() {
      return this.filterSamples(this.samples.errors || []);
    },
  },
  methods: {
    async api(path, options = {}) {
      const response = await fetch(path, {
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          ...(options.headers || {}),
        },
        ...options,
      });
      const text = await response.text();
      let payload = null;
      if (text) {
        try {
          payload = JSON.parse(text);
        } catch (error) {
          throw new Error(text);
        }
      }
      if (!response.ok) {
        throw new Error(payload?.message || `HTTP ${response.status}`);
      }
      return payload;
    },
    async login() {
      this.loginLoading = true;
      try {
        await this.api('/api/admin/login', {
          method: 'POST',
          body: JSON.stringify({ password: this.loginPassword }),
        });
        this.loginVisible = false;
        this.loginPassword = '';
        ElementPlus.ElMessage.success('登录成功');
        await this.refreshAll();
        this.startPolling();
      } catch (error) {
        ElementPlus.ElMessage.error(error.message);
      } finally {
        this.loginLoading = false;
      }
    },
    async logout() {
      try {
        await this.api('/api/admin/logout', { method: 'POST', body: '{}' });
      } catch (error) {
        // ignore
      }
      this.stopPolling();
      this.loginVisible = true;
    },
    async refreshAll() {
      this.loading = true;
      try {
        const [runtime, config, summary, scenarioStats, samples, history] = await Promise.all([
          this.api('/api/runtime'),
          this.api('/api/config'),
          this.api('/api/stats/summary'),
          this.api('/api/stats/scenarios'),
          this.api('/api/stats/samples'),
          this.api('/api/history'),
        ]);
        this.runtime = runtime;
        this.summary = summary;
        this.scenarioStats = scenarioStats;
        this.samples = samples;
        this.history = history;
        this.setConfig(config);
      } catch (error) {
        if (error.message.includes('unauthorized')) {
          this.loginVisible = true;
          this.stopPolling();
          return;
        }
        ElementPlus.ElMessage.error(error.message);
      } finally {
        this.loading = false;
      }
    },
    setConfig(config) {
      this.config = config;
      this.targetHeadersText = this.prettySample(config.target?.headers || {});
      this.maskHeadersText = (config.sampling?.mask_headers || []).join(', ');
    },
    async saveConfig() {
      this.savingConfig = true;
      try {
        const payload = JSON.parse(JSON.stringify(this.config));
        payload.target.headers = this.parseJSONText(this.targetHeadersText, 'Target Headers');
        payload.sampling.mask_headers = this.parseCommaList(this.maskHeadersText);
        await this.api('/api/config', { method: 'PUT', body: JSON.stringify(payload) });
        ElementPlus.ElMessage.success('配置已保存');
        await this.refreshAll();
      } catch (error) {
        ElementPlus.ElMessage.error(error.message);
      } finally {
        this.savingConfig = false;
      }
    },
    async startRun() {
      this.runActionLoading = true;
      try {
        await this.saveConfig();
        await this.api('/api/run/start', { method: 'POST', body: '{}' });
        ElementPlus.ElMessage.success('压测已启动');
        await this.refreshAll();
      } catch (error) {
        ElementPlus.ElMessage.error(error.message);
      } finally {
        this.runActionLoading = false;
      }
    },
    async stopRun() {
      this.runActionLoading = true;
      try {
        await this.api('/api/run/stop', { method: 'POST', body: '{}' });
        ElementPlus.ElMessage.success('已发送停止请求');
        await this.refreshAll();
      } catch (error) {
        ElementPlus.ElMessage.error(error.message);
      } finally {
        this.runActionLoading = false;
      }
    },
    addScenario() {
      this.editingScenarioIndex = -1;
      this.scenarioDraft = this.makeScenarioDraft({
        id: `scenario-${Date.now()}`,
        name: 'New Scenario',
        enabled: true,
        preset: 'raw_http',
        mode: 'single',
        weight: 1,
        method: 'POST',
        path: '/v1/chat/completions',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
        expected_statuses: [200],
        extractors: { task_id_path: 'id', task_status_path: 'status' },
        task_flow: {
          submit_request: { method: 'POST', path: '/v1/videos', headers: { 'Content-Type': 'application/json' }, body: '{}', expected_statuses: [200] },
          poll_request: { method: 'GET', path_template: '/v1/videos/{task_id}', headers: {}, expected_statuses: [200] },
          success_values: ['completed'],
          failure_values: ['failed'],
          poll_interval_ms: 1000,
          max_polls: 20,
        },
      });
      this.scenarioDrawerVisible = true;
    },
    editScenario(index) {
      this.editingScenarioIndex = index;
      this.scenarioDraft = this.makeScenarioDraft(this.config.scenarios[index]);
      this.scenarioDrawerVisible = true;
    },
    removeScenario(index) {
      this.config.scenarios.splice(index, 1);
    },
    makeScenarioDraft(source) {
      const clone = JSON.parse(JSON.stringify(source));
      clone.headersText = this.prettySample(clone.headers || {});
      clone.expectedStatusesText = (clone.expected_statuses || []).join(', ');
      clone.taskFlowSubmitHeadersText = this.prettySample(clone.task_flow?.submit_request?.headers || {});
      clone.taskFlowSubmitStatusesText = (clone.task_flow?.submit_request?.expected_statuses || []).join(', ');
      clone.taskFlowPollHeadersText = this.prettySample(clone.task_flow?.poll_request?.headers || {});
      clone.taskFlowPollStatusesText = (clone.task_flow?.poll_request?.expected_statuses || []).join(', ');
      clone.taskFlowSuccessText = (clone.task_flow?.success_values || []).join(', ');
      clone.taskFlowFailureText = (clone.task_flow?.failure_values || []).join(', ');
      return clone;
    },
    commitScenarioDraft() {
      try {
        const draft = JSON.parse(JSON.stringify(this.scenarioDraft));
        draft.headers = this.parseJSONText(draft.headersText || '{}', 'Scenario Headers');
        draft.expected_statuses = this.parseNumberList(draft.expectedStatusesText);
        draft.task_flow.submit_request.headers = this.parseJSONText(draft.taskFlowSubmitHeadersText || '{}', 'Submit Headers');
        draft.task_flow.submit_request.expected_statuses = this.parseNumberList(draft.taskFlowSubmitStatusesText);
        draft.task_flow.poll_request.headers = this.parseJSONText(draft.taskFlowPollHeadersText || '{}', 'Poll Headers');
        draft.task_flow.poll_request.expected_statuses = this.parseNumberList(draft.taskFlowPollStatusesText);
        draft.task_flow.success_values = this.parseCommaList(draft.taskFlowSuccessText);
        draft.task_flow.failure_values = this.parseCommaList(draft.taskFlowFailureText);
        delete draft.headersText;
        delete draft.expectedStatusesText;
        delete draft.taskFlowSubmitHeadersText;
        delete draft.taskFlowSubmitStatusesText;
        delete draft.taskFlowPollHeadersText;
        delete draft.taskFlowPollStatusesText;
        delete draft.taskFlowSuccessText;
        delete draft.taskFlowFailureText;
        if (this.editingScenarioIndex >= 0) {
          this.config.scenarios.splice(this.editingScenarioIndex, 1, draft);
        } else {
          this.config.scenarios.push(draft);
        }
        this.scenarioDrawerVisible = false;
      } catch (error) {
        ElementPlus.ElMessage.error(error.message);
      }
    },
    filterSamples(items) {
      if (!this.sampleFilter) return items;
      return items.filter((item) => item.scenario_id === this.sampleFilter);
    },
    openSample(row) {
      this.selectedSample = row;
      this.sampleDrawerVisible = true;
    },
    async loadHistory() {
      try {
        this.history = await this.api('/api/history');
      } catch (error) {
        ElementPlus.ElMessage.error(error.message);
      }
    },
    async openHistory(row) {
      try {
        this.selectedHistory = await this.api(`/api/history/${encodeURIComponent(row.run_id)}`);
        this.historyDrawerVisible = true;
      } catch (error) {
        ElementPlus.ElMessage.error(error.message);
      }
    },
    startPolling() {
      this.stopPolling();
      this.pollTimer = setInterval(() => {
        this.refreshAll();
      }, 2500);
    },
    stopPolling() {
      if (this.pollTimer) {
        clearInterval(this.pollTimer);
        this.pollTimer = null;
      }
    },
    parseJSONText(text, label) {
      const trimmed = (text || '').trim();
      if (!trimmed) return {};
      const parsed = JSON.parse(trimmed);
      if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') {
        throw new Error(`${label} 必须是 JSON 对象`);
      }
      return parsed;
    },
    parseCommaList(text) {
      return (text || '')
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean);
    },
    parseNumberList(text) {
      const values = this.parseCommaList(text);
      if (!values.length) return [200];
      return values.map((value) => Number(value)).filter((value) => Number.isFinite(value));
    },
    formatFloat(value) {
      return Number(value || 0).toFixed(2);
    },
    formatDuration(value) {
      return `${Number(value || 0).toFixed(2)}ms`;
    },
    formatDateTime(value) {
      if (!value) return '-';
      return new Date(value).toLocaleString();
    },
    prettySample(value) {
      try {
        return JSON.stringify(value, null, 2);
      } catch (error) {
        return String(value || '');
      }
    },
  },
  async mounted() {
    try {
      await this.refreshAll();
      this.loginVisible = false;
      this.startPolling();
    } catch (error) {
      this.loginVisible = true;
    }
  },
  beforeUnmount() {
    this.stopPolling();
  },
}).use(ElementPlus).mount('#app');
