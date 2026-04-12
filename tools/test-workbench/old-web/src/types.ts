export interface IntRange {
  min: number;
  max: number;
}

export interface FloatRange {
  min: number;
  max: number;
}

export interface GlobalRandomConfig {
  mode: 'true_random' | 'seeded';
  seed: number;
  latency_ms: IntRange;
  error_rate: number;
  too_many_requests_rate: number;
  server_error_rate: number;
  timeout_rate: number;
  default_token_length: IntRange;
  default_stream_chunks: IntRange;
}

export interface TextBehaviorConfig {
  text_tokens: IntRange;
  allow_stream: boolean;
  usage_probability: number;
  tool_call_probability: number;
  tool_call_count: IntRange;
  tool_arguments_bytes: IntRange;
  mcp_tool_probability: number;
  final_answer_probability: number;
  system_fingerprint_probability: number;
}

export interface ImageBehaviorConfig {
  sizes: string[];
  response_url_rate: number;
  image_count: IntRange;
  image_bytes: IntRange;
  watermark_texts: string[];
  watermark_rate: number;
  background_palette: string[];
}

export interface VideoBehaviorConfig {
  durations_seconds: FloatRange;
  resolutions: string[];
  fps: IntRange;
  poll_interval_ms: IntRange;
  failure_rate: number;
  video_bytes: IntRange;
  progress_jitter: IntRange;
}

export interface MockProfileConfig {
  models: string[];
  enable_pprof: boolean;
  pprof_port: number;
  random: GlobalRandomConfig;
  chat: TextBehaviorConfig;
  responses: TextBehaviorConfig;
  images: ImageBehaviorConfig;
  videos: VideoBehaviorConfig;
}

export interface SamplingConfig {
  max_request_samples: number;
  max_error_samples: number;
  max_body_bytes: number;
  mask_headers: string[];
}

export interface RunProfileConfig {
  total_concurrency: number;
  ramp_up_sec: number;
  duration_sec: number;
  max_requests: number;
  request_timeout_ms: number;
  sampling: SamplingConfig;
}

export interface ExtractorConfig {
  task_id_path: string;
  task_status_path: string;
}

export interface ScenarioRequestConfig {
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  expected_statuses: number[];
}

export interface PollRequestConfig {
  method: string;
  path_template: string;
  headers: Record<string, string>;
  expected_statuses: number[];
}

export interface TaskFlowConfig {
  submit_request: ScenarioRequestConfig;
  poll_request: PollRequestConfig;
  success_values: string[];
  failure_values: string[];
  poll_interval_ms: number;
  max_polls: number;
}

export interface ScenarioConfig {
  id: string;
  name: string;
  enabled: boolean;
  preset: string;
  mode: 'single' | 'sse' | 'task_flow';
  weight: number;
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  expected_statuses: number[];
  extractors: ExtractorConfig;
  task_flow: TaskFlowConfig;
}

export interface RuntimeLoadTarget {
  base_url: string;
  headers: Record<string, string>;
  insecure_skip_verify: boolean;
}

export interface RunExecutionConfig {
  target: RuntimeLoadTarget;
  run: RunProfileConfig;
  scenarios: ScenarioConfig[];
}

export interface Project {
  id: string;
  name: string;
  description: string;
  is_default: boolean;
}

export interface Environment {
  id: string;
  project_id: string;
  name: string;
  target_type: 'internal_mock' | 'external_http';
  external_base_url: string;
  default_headers: Record<string, string>;
  insecure_skip_verify: boolean;
  mock_bind_host: string;
  mock_port: number;
  mock_require_auth: boolean;
  mock_auth_token: string;
  auto_start: boolean;
  default_mock_profile_id: string;
  default_run_profile_id: string;
}

export interface MockProfile {
  id: string;
  project_id: string;
  name: string;
  config: MockProfileConfig;
}

export interface RunProfile {
  id: string;
  project_id: string;
  name: string;
  config: RunProfileConfig;
}

export interface Scenario {
  id: string;
  project_id: string;
  name: string;
  config: ScenarioConfig;
}

export interface MockListenerRuntime {
  environment_id: string;
  name: string;
  status: string;
  local_base_url: string;
  listen_address: string;
  require_auth: boolean;
  profile_id: string;
  summary: {
    total_requests: number;
    errors: number;
    current_qps: number;
    active_requests: number;
    active_sse: number;
    video_tasks: number;
    status_codes: Record<string, number>;
  };
}

export interface LoadRunRuntime {
  run_id: string;
  project_id: string;
  environment_id: string;
  run_profile_id: string;
  status: string;
  started_at: string;
  target_base_url: string;
  summary: {
    total_requests: number;
    successes: number;
    errors: number;
    timeouts: number;
    p95_ms: number;
    current_tps: number;
  };
}

export interface RunListItem {
  id: string;
  project_id: string;
  environment_id: string;
  run_profile_id: string;
  status: string;
  started_at: string;
  finished_at?: string;
  total_requests: number;
  successes: number;
  errors: number;
  timeouts: number;
  p95_ms: number;
}

export interface RunRecord {
  id: string;
  project_id: string;
  environment_id: string;
  run_profile_id: string;
  status: string;
  config: RunExecutionConfig;
  summary: {
    total_requests: number;
    successes: number;
    errors: number;
    timeouts: number;
    current_tps: number;
    average_ms: number;
    p95_ms: number;
    status_codes: Record<string, number>;
    error_kinds: Record<string, number>;
  };
  scenarios: Array<{
    scenario_id: string;
    name: string;
    mode: string;
    requests: number;
    successes: number;
    errors: number;
    p95_ms: number;
  }>;
  samples: {
    requests: Array<Record<string, unknown>>;
    errors: Array<Record<string, unknown>>;
  };
}

export interface RouteStatsItem {
  route: string;
  requests: number;
  errors: number;
  p95_ms: number;
}

export interface EventsResponse {
  requests: Array<Record<string, unknown>>;
  errors: Array<Record<string, unknown>>;
  videos: Array<Record<string, unknown>>;
}
