import type { MockProfileConfig, RunProfileConfig, ScenarioConfig } from '../../types';

export function cloneValue<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function safePretty(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

export function formatStatus(value: string): string {
  switch (value) {
    case 'queued':
      return '排队中';
    case 'running':
      return '运行中';
    case 'completed':
      return '已完成';
    case 'failed':
      return '失败';
    case 'aborted':
      return '已中止';
    case 'stopped':
      return '已停止';
    default:
      return value || '未知';
  }
}

export function statusColor(status: string) {
  switch (status) {
    case 'running':
      return 'green';
    case 'completed':
      return 'blue';
    case 'queued':
      return 'orange';
    case 'failed':
      return 'red';
    case 'aborted':
      return 'grey';
    case 'stopped':
      return 'grey';
    default:
      return 'grey';
  }
}

export function formatTargetType(value: string): string {
  switch (value) {
    case 'internal_mock':
      return '内置 Mock';
    case 'external_http':
      return '外部 HTTP';
    default:
      return value;
  }
}

export function formatScenarioMode(value: string): string {
  switch (value) {
    case 'single':
      return '单次请求';
    case 'sse':
      return '流式 SSE';
    case 'task_flow':
      return '任务轮询';
    default:
      return value;
  }
}

export function createDefaultMockProfileConfig(): MockProfileConfig {
  return {
    models: ['gpt-4o-mini', 'gpt-4o', 'gpt-image-1', 'sora-mini'],
    enable_pprof: false,
    pprof_port: 18885,
    random: {
      mode: 'true_random',
      seed: 20260411,
      latency_ms: { min: 20, max: 150 },
      error_rate: 0.02,
      too_many_requests_rate: 0.01,
      server_error_rate: 0.01,
      timeout_rate: 0.005,
      default_token_length: { min: 48, max: 256 },
      default_stream_chunks: { min: 2, max: 8 }
    },
    chat: {
      text_tokens: { min: 48, max: 256 },
      allow_stream: true,
      usage_probability: 0.95,
      tool_call_probability: 0.35,
      tool_call_count: { min: 1, max: 3 },
      tool_arguments_bytes: { min: 64, max: 384 },
      mcp_tool_probability: 0.15,
      final_answer_probability: 0.85,
      system_fingerprint_probability: 0.75
    },
    responses: {
      text_tokens: { min: 48, max: 256 },
      allow_stream: true,
      usage_probability: 0.95,
      tool_call_probability: 0.4,
      tool_call_count: { min: 1, max: 3 },
      tool_arguments_bytes: { min: 64, max: 512 },
      mcp_tool_probability: 0.2,
      final_answer_probability: 0.8,
      system_fingerprint_probability: 0.6
    },
    images: {
      sizes: ['512x512', '768x768', '1024x1024', '1024x1536'],
      response_url_rate: 0.7,
      image_count: { min: 1, max: 4 },
      image_bytes: { min: 12000, max: 120000 },
      watermark_texts: ['mock', 'new-api', 'simulated', 'fake upstream'],
      watermark_rate: 0.85,
      background_palette: ['#0F172A', '#155E75', '#4A044E', '#7C2D12', '#1D4ED8']
    },
    videos: {
      durations_seconds: { min: 3, max: 12 },
      resolutions: ['640x360', '960x540', '1280x720'],
      fps: { min: 12, max: 30 },
      poll_interval_ms: { min: 400, max: 2200 },
      failure_rate: 0.15,
      video_bytes: { min: 90000, max: 350000 },
      progress_jitter: { min: 1, max: 12 }
    }
  };
}

export function createDefaultRunProfileConfig(): RunProfileConfig {
  return {
    total_concurrency: 20,
    ramp_up_sec: 5,
    duration_sec: 30,
    max_requests: 0,
    request_timeout_ms: 30000,
    sampling: {
      max_request_samples: 100,
      max_error_samples: 100,
      max_body_bytes: 4096,
      mask_headers: ['authorization', 'x-api-key', 'api-key', 'x-mock-admin-token', 'x-loadtest-admin-token']
    }
  };
}

function createDefaultPollRequest() {
  return {
    method: 'GET',
    path_template: '/v1/videos/{task_id}',
    headers: {},
    expected_statuses: [200]
  };
}

export function createDefaultScenarioConfig(): ScenarioConfig {
  return {
    id: `scenario-${Date.now()}`,
    name: '新场景',
    enabled: true,
    preset: 'raw_http',
    mode: 'single',
    weight: 1,
    method: 'GET',
    path: '/v1/models',
    headers: {},
    body: '',
    expected_statuses: [200],
    extractors: {
      task_id_path: '',
      task_status_path: ''
    },
    task_flow: {
      submit_request: {
        method: 'POST',
        path: '/v1/videos',
        headers: {},
        body: '',
        expected_statuses: [200]
      },
      poll_request: createDefaultPollRequest(),
      success_values: ['completed', 'succeeded', 'success'],
      failure_values: ['failed', 'error', 'cancelled'],
      poll_interval_ms: 1000,
      max_polls: 20
    }
  };
}

function getPresetTemplate(preset: string): Partial<ScenarioConfig> | null {
  switch (preset) {
    case 'new_api_chat':
      return {
        mode: 'sse',
        method: 'POST',
        path: '/v1/chat/completions',
        body: '{"model":"gpt-4o-mini","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hello from load tester"}]}',
        expected_statuses: [200]
      };
    case 'new_api_responses':
      return {
        mode: 'sse',
        method: 'POST',
        path: '/v1/responses',
        body: '{"model":"gpt-4o-mini","stream":true,"input":[{"role":"user","content":"hello from load tester"}]}',
        expected_statuses: [200]
      };
    case 'new_api_images':
      return {
        mode: 'single',
        method: 'POST',
        path: '/v1/images/generations',
        body: '{"model":"gpt-image-1","prompt":"a calm geometric poster","size":"512x512"}',
        expected_statuses: [200]
      };
    case 'new_api_videos':
      return {
        mode: 'task_flow',
        extractors: {
          task_id_path: 'id',
          task_status_path: 'status'
        },
        task_flow: {
          submit_request: {
            method: 'POST',
            path: '/v1/videos',
            headers: {},
            body: '{"model":"sora-mini","prompt":"waves over a lake","duration":2.5,"size":"640x360","fps":12}',
            expected_statuses: [200]
          },
          poll_request: {
            method: 'GET',
            path_template: '/v1/videos/{task_id}',
            headers: {},
            expected_statuses: [200]
          },
          success_values: ['completed', 'succeeded', 'success'],
          failure_values: ['failed', 'error', 'cancelled'],
          poll_interval_ms: 1000,
          max_polls: 20
        }
      };
    default:
      return null;
  }
}

export function applyScenarioPreset(current: ScenarioConfig, preset: string): ScenarioConfig {
  const next = cloneValue(current);
  next.preset = preset;
  if (preset === 'raw_http') {
    return next;
  }

  const presetTemplate = getPresetTemplate(preset);
  if (!presetTemplate) {
    return next;
  }

  return {
    ...next,
    ...presetTemplate,
    extractors: presetTemplate.extractors ? cloneValue(presetTemplate.extractors) : next.extractors,
    task_flow: presetTemplate.task_flow ? cloneValue(presetTemplate.task_flow) : next.task_flow
  };
}
