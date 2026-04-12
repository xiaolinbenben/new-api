const { createApp, ref, computed, onMounted, onUnmounted, watch, nextTick } = Vue;
const { ElMessage } = ElementPlus;

// Element Plus Icons registration
const icons = {
  DataLine: ElementPlusIconsVue.DataLine,
  Setting: ElementPlusIconsVue.Setting,
  Cpu: ElementPlusIconsVue.Cpu,
  Dice: ElementPlusIconsVue.Dice,
  ChatDotRound: ElementPlusIconsVue.ChatDotRound,
  Message: ElementPlusIconsVue.Message,
  Picture: ElementPlusIconsVue.Picture,
  VideoCamera: ElementPlusIconsVue.VideoCamera,
  Check: ElementPlusIconsVue.Check,
  VideoPlay: ElementPlusIconsVue.VideoPlay,
  VideoPause: ElementPlusIconsVue.VideoPause,
  RefreshRight: ElementPlusIconsVue.RefreshRight,
  Sunny: ElementPlusIconsVue.Sunny,
  Moon: ElementPlusIconsVue.Moon,
  InfoFilled: ElementPlusIconsVue.InfoFilled
};

createApp({
  setup() {
    // ==================== State ====================
    const activeTab = ref('dashboard');
    const actionStatus = ref('');
    const saving = ref(false);
    const isDarkMode = ref(false);
    const runtime = ref(null);
    const summary = ref(null);
    const qpsHistory = ref([]);
    const maxHistoryPoints = 60;

    // Chart refs
    const qpsChart = ref(null);
    const statusChart = ref(null);
    const routeChart = ref(null);
    const latencyChart = ref(null);

    // Chart instances
    let charts = {};

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

    // ==================== Computed ====================
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

    const dashboardStats = computed(() => {
      const s = summary.value || {};
      const r = runtime.value?.worker || {};
      return [
        { key: 'total_requests', label: '总请求数', value: s.total_requests || 0, color: '#409EFF' },
        { key: 'qps', label: 'QPS', value: (s.current_qps || 0).toFixed(2), color: '#67C23A', trend: '实时' },
        { key: 'error_rate', label: '错误率', value: ((s.error_rate || 0) * 100).toFixed(2) + '%', color: (s.error_rate || 0) > 0.05 ? '#F56C6C' : '#67C23A' },
        { key: 'active_requests', label: '活跃请求', value: s.active_requests || 0, color: '#E6A23C' },
        { key: 'active_sse', label: '活跃 SSE', value: s.active_sse || 0, color: '#909399' },
        { key: 'video_tasks', label: '视频任务', value: s.video_tasks || 0, color: '#409EFF' }
      ];
    });

    // ==================== Dark Mode (Element Plus Official) ====================
    function initDarkMode() {
      const saved = localStorage.getItem('mock_upstream_dark_mode');
      if (saved !== null) {
        isDarkMode.value = saved === 'true';
      } else {
        isDarkMode.value = window.matchMedia('(prefers-color-scheme: dark)').matches;
      }
      applyDarkMode();
    }

    function toggleDarkMode() {
      isDarkMode.value = !isDarkMode.value;
      localStorage.setItem('mock_upstream_dark_mode', isDarkMode.value);
      applyDarkMode();
      updateChartsTheme();
    }

    function applyDarkMode() {
      // Element Plus official dark mode: just add/remove 'dark' class on html
      const html = document.documentElement;
      if (isDarkMode.value) {
        html.classList.add('dark');
      } else {
        html.classList.remove('dark');
      }
    }

    // ==================== Charts ====================
    function initCharts() {
      if (!qpsChart.value || !statusChart.value || !routeChart.value || !latencyChart.value) return;

      // Get colors from Element Plus CSS variables
      const textColor = getComputedStyle(document.documentElement).getPropertyValue('--el-text-color-primary').trim() || '#303133';
      const axisColor = getComputedStyle(document.documentElement).getPropertyValue('--el-border-color').trim() || '#dcdfe6';
      const splitLineColor = getComputedStyle(document.documentElement).getPropertyValue('--el-border-color-lighter').trim() || '#ebeef5';
      const primaryColor = getComputedStyle(document.documentElement).getPropertyValue('--el-color-primary').trim() || '#409EFF';
      const successColor = getComputedStyle(document.documentElement).getPropertyValue('--el-color-success').trim() || '#67C23A';
      const warningColor = getComputedStyle(document.documentElement).getPropertyValue('--el-color-warning').trim() || '#E6A23C';
      const bgColor = getComputedStyle(document.documentElement).getPropertyValue('--el-bg-color').trim() || '#ffffff';

      // QPS Chart (Line)
      charts.qps = echarts.init(qpsChart.value);
      charts.qps.setOption({
        grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
        xAxis: { type: 'category', boundaryGap: false, data: [], axisLine: { lineStyle: { color: axisColor } }, axisLabel: { color: textColor }, splitLine: { lineStyle: { color: splitLineColor } } },
        yAxis: { type: 'value', min: 0, axisLine: { lineStyle: { color: axisColor } }, axisLabel: { color: textColor }, splitLine: { lineStyle: { color: splitLineColor } } },
        series: [{
          name: 'QPS',
          type: 'line',
          smooth: true,
          areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: primaryColor + '4d' }, { offset: 1, color: primaryColor + '0d' }]) },
          lineStyle: { color: primaryColor, width: 2 },
          itemStyle: { color: primaryColor },
          data: []
        }]
      });

      // Status Code Chart (Pie)
      charts.status = echarts.init(statusChart.value);
      charts.status.setOption({
        tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
        legend: { orient: 'vertical', left: 'left', textStyle: { color: textColor } },
        series: [{
          type: 'pie',
          radius: ['40%', '70%'],
          avoidLabelOverlap: false,
          itemStyle: { borderRadius: 6, borderColor: bgColor, borderWidth: 2 },
          label: { show: true, formatter: '{b}\n{c}', color: textColor },
          data: []
        }]
      });

      // Route Chart (Bar)
      charts.route = echarts.init(routeChart.value);
      charts.route.setOption({
        grid: { left: '3%', right: '4%', bottom: '15%', top: '10%', containLabel: true },
        xAxis: { type: 'category', data: [], axisLabel: { rotate: 30, color: textColor }, axisLine: { lineStyle: { color: axisColor } }, splitLine: { lineStyle: { color: splitLineColor } } },
        yAxis: { type: 'value', axisLine: { lineStyle: { color: axisColor } }, axisLabel: { color: textColor }, splitLine: { lineStyle: { color: splitLineColor } } },
        series: [{
          type: 'bar',
          data: [],
          itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: successColor }, { offset: 1, color: successColor + 'aa' }]), borderRadius: [4, 4, 0, 0] }
        }]
      });

      // Latency Chart (Bar with P50/P95)
      charts.latency = echarts.init(latencyChart.value);
      charts.latency.setOption({
        tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
        legend: { data: ['平均', 'P50', 'P95'], textStyle: { color: textColor } },
        grid: { left: '3%', right: '4%', bottom: '15%', top: '15%', containLabel: true },
        xAxis: { type: 'category', data: [], axisLabel: { rotate: 30, color: textColor }, axisLine: { lineStyle: { color: axisColor } }, splitLine: { lineStyle: { color: splitLineColor } } },
        yAxis: { type: 'value', name: 'ms', axisLine: { lineStyle: { color: axisColor } }, axisLabel: { color: textColor }, splitLine: { lineStyle: { color: splitLineColor } } },
        series: [
          { name: '平均', type: 'bar', data: [], itemStyle: { color: primaryColor } },
          { name: 'P50', type: 'bar', data: [], itemStyle: { color: successColor } },
          { name: 'P95', type: 'bar', data: [], itemStyle: { color: warningColor } }
        ]
      });
    }

    function updateCharts() {
      if (!charts.qps || !charts.status || !charts.route || !charts.latency) return;

      // Update QPS history
      const now = new Date().toLocaleTimeString();
      qpsHistory.value.push({ time: now, value: summary.value?.current_qps || 0 });
      if (qpsHistory.value.length > maxHistoryPoints) {
        qpsHistory.value.shift();
      }

      charts.qps.setOption({
        xAxis: { data: qpsHistory.value.map(h => h.time) },
        series: [{ data: qpsHistory.value.map(h => h.value) }]
      });

      // Update Status Code Pie
      const statusCodes = summary.value?.status_codes || {};
      const statusData = Object.entries(statusCodes).map(([code, count]) => ({
        name: code,
        value: count,
        itemStyle: {
          color: code.startsWith('2') ? '#67C23A' : code.startsWith('4') ? '#E6A23C' : code.startsWith('5') ? '#F56C6C' : '#909399'
        }
      }));
      charts.status.setOption({ series: [{ data: statusData }] });

      // Update Route Chart
      const routeData = routesTableData.value.slice(0, 10);
      charts.route.setOption({
        xAxis: { data: routeData.map(r => r.route.replace('/v1/', '')) },
        series: [{ data: routeData.map(r => r.requests) }]
      });

      // Update Latency Chart
      charts.latency.setOption({
        xAxis: { data: routeData.map(r => r.route.replace('/v1/', '')) },
        series: [
          { data: routeData.map(r => r.average_ms) },
          { data: routeData.map(r => r.p50_ms) },
          { data: routeData.map(r => r.p95_ms) }
        ]
      });
    }

    function updateChartsTheme() {
      if (!charts.qps) return;
      // Get colors from Element Plus CSS variables
      const textColor = getComputedStyle(document.documentElement).getPropertyValue('--el-text-color-primary').trim() || '#303133';
      const axisColor = getComputedStyle(document.documentElement).getPropertyValue('--el-border-color').trim() || '#dcdfe6';
      const splitLineColor = getComputedStyle(document.documentElement).getPropertyValue('--el-border-color-lighter').trim() || '#ebeef5';

      Object.values(charts).forEach(chart => {
        chart.setOption({
          xAxis: { axisLine: { lineStyle: { color: axisColor } }, axisLabel: { color: textColor }, splitLine: { lineStyle: { color: splitLineColor } } },
          yAxis: { axisLine: { lineStyle: { color: axisColor } }, axisLabel: { color: textColor }, splitLine: { lineStyle: { color: splitLineColor } } },
          legend: { textStyle: { color: textColor } }
        });
      });
    }

    function resizeCharts() {
      Object.values(charts).forEach(chart => chart && chart.resize());
    }

    // ==================== API ====================
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
        summary.value = await fetchAPI('/api/stats/summary');
        updateCharts();
      } catch (e) {
        console.error('Failed to load summary:', e);
      }
    }

    async function loadRoutes() {
      try {
        const data = await fetchAPI('/api/stats/routes');
        routesTableData.value = (data.items || []).map(item => ({
          route: item.route,
          requests: item.requests,
          successes: item.successes,
          errors: item.errors,
          active: item.active,
          average_ms: item.average_ms,
          p50_ms: item.p50_ms,
          p95_ms: item.p95_ms,
          last_status: item.last_status
        }));
        updateCharts();
      } catch (e) {
        console.error('Failed to load routes:', e);
      }
    }

    async function loadEvents() {
      try {
        const data = await fetchAPI('/api/stats/events');
        errorsTableData.value = (data.errors || []).slice(0, 10).map(item => ({
          timestamp: new Date(item.timestamp).toLocaleTimeString(),
          route: item.route,
          status_code: item.status_code || '-',
          error: item.error || '-'
        }));
        requestsTableData.value = (data.requests || []).slice(0, 10).map(item => ({
          timestamp: new Date(item.timestamp).toLocaleTimeString(),
          method: item.method || 'POST',
          route: item.route,
          status_code: item.status_code || '-',
          latency_ms: item.latency_ms || '-'
        }));
        videosTableData.value = (data.videos || []).slice(0, 10).map(item => ({
          timestamp: new Date(item.timestamp).toLocaleTimeString(),
          task_id: item.task_id || item.id || '-',
          status: item.status || '-',
          resolution: item.resolution || '-',
          fps: item.fps || '-'
        }));
      } catch (e) {
        console.error('Failed to load events:', e);
      }
    }

    async function refreshAll() {
      try {
        actionStatus.value = '刷新中...';
        await Promise.all([loadConfig(), loadRuntime(), loadSummary(), loadRoutes(), loadEvents()]);
        actionStatus.value = '';
        ElMessage.success('刷新成功');
      } catch (e) {
        actionStatus.value = '';
        ElMessage.error('刷新失败: ' + e.message);
      }
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
        saving.value = true;
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
        saving.value = false;
        actionStatus.value = '';
      }
    }

    async function resetStats() {
      try {
        actionStatus.value = '重置中...';
        await fetchAPI('/api/stats/reset', { method: 'POST' });
        ElMessage.success('统计已重置');
        qpsHistory.value = [];
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
        loadRoutes();
        loadEvents();
      }, 2000);
    }

    function stopPolling() {
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    }

    // ==================== Lifecycle ====================
    onMounted(() => {
      initDarkMode();
      refreshAll();
      nextTick(() => {
        initCharts();
        updateCharts();
      });
      startPolling();
      window.addEventListener('resize', resizeCharts);
    });

    onUnmounted(() => {
      stopPolling();
      window.removeEventListener('resize', resizeCharts);
      Object.values(charts).forEach(chart => chart && chart.dispose());
    });

    // Watch for tab changes to init charts
    watch(activeTab, (newTab) => {
      if (newTab === 'dashboard') {
        nextTick(() => {
          initCharts();
          updateCharts();
        });
      }
    });

    return {
      activeTab,
      actionStatus,
      saving,
      isDarkMode,
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
      qpsChart,
      statusChart,
      routeChart,
      latencyChart,
      dashboardStats,
      workerStatusText,
      workerStatusType,
      toggleDarkMode,
      refreshAll,
      workerAction,
      saveConfig,
      resetStats,
      loadRoutes
    };
  }
}).use(ElementPlus).mount('#app');
