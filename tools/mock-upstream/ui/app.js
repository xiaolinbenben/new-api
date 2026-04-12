const { createApp, ref, computed, onMounted, onUnmounted } = Vue;
const { ElMessage } = ElementPlus;

createApp({
  setup() {
    const activeTab = ref('overview');
    const actionStatus = ref('');
    const runtime = ref(null);
    const summary = ref(null);
    const config = ref({
      worker: { port: 18081, pprof_port: 18085, require_auth: false, enable_pprof: false, management_token: '', models: [] },
      random: { mode: 'true_random', seed: 20260411, latency_ms: { min: 20, max: 150 }, error_rate: 0.02, too_many_requests_rate: 0.01, server_error_rate: 0.01, timeout_rate: 0.005, default_token_length: { min: 48, max: 256 }, default_stream_chunks: { min: 2, max: 8 } },
      chat: { text_tokens: { min: 48, max: 256 }, allow_stream: true, usage_probability: 0.95, tool_call_probability: 0.35, tool_call_count: { min: 1, max: 3 }, tool_arguments_bytes: { min: 64, max: 384 }, mcp_tool_probability: 0.15, final_answer_probability: 0.85, system_fingerprint_chance: 0.75 },
      responses: { text_tokens: { min: 48, max: 256 }, allow_stream: true, usage_probability: 0.95, tool_call_probability: 0.40, tool_call_count: { min: 1, max: 3 }, tool_arguments_bytes: { min: 64, max: 512 }, mcp_tool_probability: 0.20, final_answer_probability: 0.80, system_fingerprint_chance: 0.60 },
      images: { sizes: [], response_url_rate: 0.70, image_count: { min: 1, max: 4 }, image_bytes: { min: 12000, max: 120000 }, watermark_texts: [], watermark_rate: 0.85, background_palette: [] },
      videos: { durations_seconds: { min: 3, max: 12 }, resolutions: [], fps: { min: 12, max: 30 }, poll_interval_ms: { min: 400, max: 2200 }, failure_rate: 0.15, video_bytes: { min: 90000, max: 350000 }, progress_jitter: { min: 1, max: 12 } }
    });
    
    const modelsText = ref('');
    const imageSizesText = ref('');
    const imageWatermarksText = ref('');
    const imagePaletteText = ref('');
    const videoResolutionsText = ref('');
    
    const routesTableData = ref([]);
    const errorsTableData = ref([]);
    const requestsTableData = ref([]);
    const videosTableData = ref([]);
    
    let pollTimer = null;

    const tabs = [
      { key: 'overview', label: '总览', title: '总览', subtitle: '查看 worker 状态、吞吐和主要运行指标' },
      { key: 'service', label: '服务控制', title: '服务控制', subtitle: '管理 worker 生命周期、端口和鉴权设置' },
      { key: 'random', label: '全局随机', title: '全局随机', subtitle: '调整随机模式、延迟、错误率和 Token 范围' },
      { key: 'conversation', label: '对话配置', title: '对话配置', subtitle: '配置 Chat 和 Responses 的流式、Tool 调用参数' },
      { key: 'media', label: '媒体配置', title: '媒体配置', subtitle: '配置图片生成和视频任务参数' },
      { key: 'data', label: '数据统计', title: '数据统计', subtitle: '查看路由统计、最近错误、请求和视频任务' }
    ];

    const currentTab = computed(() => {
      const tab = tabs.find(t => t.key === activeTab.value);
      return tab || { title: '总览', subtitle: '' };
    });

    const workerStatusText = computed(() => {
      const status = runtime.value?.worker?.status;
      const mapping = { running: '运行中', stopped: '已停止', starting: '启动中', unknown: '未知' };
      return mapping[status] || '未知';
    });

    const workerStatusType = computed(() => {
      const status = runtime.value?.worker?.status;
      const mapping = { running: 'success', stopped: 'danger', starting: 'warning', unknown: 'info' };
      return mapping[status] || 'info';
    });

    const sortedStatusCodes = computed(() => {
      const codes = summary.value?.status_codes || {};
      return Object.entries(codes).sort(([a], [b]) => Number(a) - Number(b));
    });

    async function fetchAPI(endpoint, options = {}) {
      const response = await fetch(endpoint, options);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      return response.json();
    }

    async function loadConfig() {
      try {
        const data = await fetchAPI('/api/config');
        config.value = data;
        modelsText.value = data.worker.models.join('\n');
        imageSizesText.value = data.images.sizes.join('\n');
        imageWatermarksText.value = data.images.watermark_texts.join('\n');
        imagePaletteText.value = data.images.background_palette.join('\n');
        videoResolutionsText.value = data.videos.resolutions.join('\n');
      } catch (e) {
        console.error('Failed to load config:', e);
      }
    }

    async function loadRuntime() {
      try {
        runtime.value = await fetchAPI('/api/runtime');
      } catch (e) {
        console.error('Failed to load runtime:', e);
      }
    }

    async function loadSummary() {
      try {
        summary.value = await fetchAPI('/api/summary');
      } catch (e) {
        console.error('Failed to load summary:', e);
      }
    }

    async function loadRoutes() {
      try {
        const data = await fetchAPI('/api/routes');
        routesTableData.value = data.items.map(item => ({
          route: item.route,
          requests: item.requests,
          success: item.success,
          errors: item.errors,
          active: item.active,
          avg_ms: item.avg_ms,
          p50: item.p50,
          p95: item.p95,
          last_status: item.last_status
        }));
      } catch (e) {
        console.error('Failed to load routes:', e);
      }
    }

    async function loadErrors() {
      try {
        const data = await fetchAPI('/api/errors');
        errorsTableData.value = data.items.map(item => ({
          time: new Date(item.timestamp).toLocaleString(),
          route: item.route,
          error: item.error
        }));
      } catch (e) {
        console.error('Failed to load errors:', e);
      }
    }

    async function loadRequests() {
      try {
        const data = await fetchAPI('/api/requests');
        requestsTableData.value = data.items.map(item => ({
          time: new Date(item.timestamp).toLocaleString(),
          route: item.route,
          model: item.model
        }));
      } catch (e) {
        console.error('Failed to load requests:', e);
      }
    }

    async function loadVideos() {
      try {
        const data = await fetchAPI('/api/videos');
        videosTableData.value = data.items.map(item => ({
          time: new Date(item.created_at).toLocaleString(),
          id: item.id,
          status: item.status
        }));
      } catch (e) {
        console.error('Failed to load videos:', e);
      }
    }

    async function refreshAll() {
      actionStatus.value = '刷新中...';
      await Promise.all([loadConfig(), loadRuntime(), loadSummary()]);
      if (activeTab.value === 'data') {
        await Promise.all([loadRoutes(), loadErrors(), loadRequests(), loadVideos()]);
      }
      actionStatus.value = '';
    }

    async function workerAction(endpoint, message) {
      try {
        actionStatus.value = '处理中...';
        await fetchAPI(endpoint, { method: 'POST' });
        ElMessage.success(message);
        await loadRuntime();
      } catch (e) {
        ElMessage.error('操作失败: ' + e.message);
      } finally {
        actionStatus.value = '';
      }
    }

    async function saveConfig() {
      try {
        actionStatus.value = '保存中...';
        const payload = {
          worker: {
            port: config.value.worker.port,
            require_auth: config.value.worker.require_auth,
            management_token: config.value.worker.management_token,
            enable_pprof: config.value.worker.enable_pprof,
            pprof_port: config.value.worker.pprof_port,
            models: modelsText.value.split('\n').map(s => s.trim()).filter(s => s)
          },
          random: config.value.random,
          chat: config.value.chat,
          responses: config.value.responses,
          images: {
            ...config.value.images,
            sizes: imageSizesText.value.split('\n').map(s => s.trim()).filter(s => s),
            watermark_texts: imageWatermarksText.value.split('\n').map(s => s.trim()).filter(s => s),
            background_palette: imagePaletteText.value.split('\n').map(s => s.trim()).filter(s => s)
          },
          videos: {
            ...config.value.videos,
            resolutions: videoResolutionsText.value.split('\n').map(s => s.trim()).filter(s => s)
          }
        };
        await fetchAPI('/api/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        ElMessage.success('配置已保存');
      } catch (e) {
        ElMessage.error('保存失败: ' + e.message);
      } finally {
        actionStatus.value = '';
      }
    }

    async function resetStats() {
      try {
        actionStatus.value = '重置中...';
        await fetchAPI('/api/stats/reset', { method: 'POST' });
        ElMessage.success('统计已重置');
        await loadRoutes();
      } catch (e) {
        ElMessage.error('重置失败: ' + e.message);
      } finally {
        actionStatus.value = '';
      }
    }

    function startPolling() {
      pollTimer = setInterval(() => {
        loadRuntime();
        loadSummary();
        if (activeTab.value === 'data') {
          loadRoutes();
        }
      }, 2000);
    }

    function stopPolling() {
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    }

    onMounted(() => {
      refreshAll();
      startPolling();
    });

    onUnmounted(() => {
      stopPolling();
    });

    return {
      activeTab,
      actionStatus,
      runtime,
      summary,
      config,
      modelsText,
      imageSizesText,
      imageWatermarksText,
      imagePaletteText,
      videoResolutionsText,
      routesTableData,
      errorsTableData,
      requestsTableData,
      videosTableData,
      tabs,
      currentTab,
      workerStatusText,
      workerStatusType,
      sortedStatusCodes,
      refreshAll,
      workerAction,
      saveConfig,
      resetStats
    };
  }
}).use(ElementPlus).mount('#app');
