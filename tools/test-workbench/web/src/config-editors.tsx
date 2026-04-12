import type { ReactNode } from 'react';
import {
  Button,
  Input,
  InputNumber,
  Select,
  Switch,
  Tag,
  TextArea,
  Typography
} from '@douyinfe/semi-ui';
import type {
  Environment,
  FloatRange,
  IntRange,
  MockProfile,
  MockProfileConfig,
  RunProfile,
  Scenario,
  ScenarioConfig,
  ScenarioRequestConfig,
  TextBehaviorConfig
} from './types';
import { applyScenarioPreset, cloneValue, formatScenarioMode, formatTargetType } from './features/workbench/helpers';

const targetTypeOptions = [
  { value: 'internal_mock', label: '内置 Mock（internal_mock）' },
  { value: 'external_http', label: '外部 HTTP（external_http）' }
] as const;

const randomModeOptions = [
  { value: 'true_random', label: '真随机' },
  { value: 'seeded', label: '固定种子' }
] as const;

const presetOptions = [
  { value: 'raw_http', label: '自定义 HTTP 请求' },
  { value: 'new_api_chat', label: '聊天补全模板' },
  { value: 'new_api_responses', label: 'Responses 模板' },
  { value: 'new_api_images', label: '图片生成模板' },
  { value: 'new_api_videos', label: '视频任务模板' }
] as const;

const scenarioModeOptions = [
  { value: 'single', label: '单次请求' },
  { value: 'sse', label: '流式 SSE' },
  { value: 'task_flow', label: '任务轮询' }
] as const;

const httpMethodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD'];

interface EnvironmentEditorProps {
  value: Environment;
  mockProfiles: MockProfile[];
  runProfiles: RunProfile[];
  onChange: (value: Environment) => void;
  onSave: () => void;
  onDelete?: () => void;
}

export function EnvironmentEditor(props: EnvironmentEditorProps) {
  const { value, mockProfiles, runProfiles, onChange, onSave, onDelete } = props;

  return (
    <div className='config-editor'>
      <EditorIntro
        title='环境配置'
        description='这里定义压测目标、默认请求头、内置 Mock 监听器以及本环境绑定的默认配置。切换不同环境后，工作台会自动基于这里的设置启动或执行。'
        extra={<Tag color='light-blue'>{formatTargetType(value.target_type)}</Tag>}
      />

      <SectionCard title='基础信息' description='先定义环境本身的名称、目标类型，以及本环境默认绑定哪一套 Mock / 压测配置。'>
        <div className='form-grid two-columns'>
          <Field label='环境名称' hint='用于列表展示、监听器面板以及运行历史关联显示。'>
            <Input value={value.name} onChange={(next) => onChange({ ...value, name: next })} placeholder='例如：本地联调环境' />
          </Field>
          <Field label='目标类型' hint='内置 Mock 会自动将压测目标指向本地模拟器；外部 HTTP 则请求你指定的上游地址。'>
            <Select
              value={value.target_type}
              optionList={targetTypeOptions.map((item) => ({ value: item.value, label: item.label }))}
              onChange={(next) => onChange({ ...value, target_type: next as Environment['target_type'] })}
            />
          </Field>
          <Field label='默认 Mock 配置' hint='启动该环境的监听器时，如果你没有额外指定，将默认使用这里绑定的 Mock 配置。'>
            <Select
              value={value.default_mock_profile_id}
              optionList={mockProfiles.map((item) => ({ value: item.id, label: item.name }))}
              onChange={(next) => onChange({ ...value, default_mock_profile_id: String(next) })}
            />
          </Field>
          <Field label='默认压测配置' hint='启动压测时，如果你没有手动切换，则优先使用这里绑定的压测参数。'>
            <Select
              value={value.default_run_profile_id}
              optionList={runProfiles.map((item) => ({ value: item.id, label: item.name }))}
              onChange={(next) => onChange({ ...value, default_run_profile_id: String(next) })}
            />
          </Field>
        </div>
      </SectionCard>

      <SectionCard title='目标地址与默认请求' description='这里控制外部 HTTP 目标地址、TLS 行为以及所有场景都会继承的默认请求头。'>
        <div className='form-grid two-columns'>
          <Field label='外部基础地址' hint='仅在“外部 HTTP”模式下生效，例如 https://example.com。内置 Mock 模式下可留空。'>
            <Input
              value={value.external_base_url}
              onChange={(next) => onChange({ ...value, external_base_url: next })}
              placeholder='https://example.com'
            />
          </Field>
          <Field label='TLS 设置' hint='访问外部 HTTPS 目标时，是否跳过证书校验。仅在开发或测试环境使用。'>
            <BooleanCard
              checked={value.insecure_skip_verify}
              label='跳过 TLS 证书校验'
              description='启用后，工作台会忽略无效、自签名或内部测试证书。'
              onChange={(checked) => onChange({ ...value, insecure_skip_verify: checked })}
            />
          </Field>
        </div>
        <KeyValueEditor
          label='默认请求头'
          hint='这里的请求头会附加到本环境所有压测请求上，适合放统一的鉴权头、调试标记或租户信息。'
          value={value.default_headers}
          keyPlaceholder='请求头名称，例如 Authorization'
          valuePlaceholder='请求头内容'
          onChange={(next) => onChange({ ...value, default_headers: next })}
        />
      </SectionCard>

      <SectionCard title='内置 Mock 监听器' description='这里决定内置模拟器监听在哪个地址、是否自动启动、以及是否要求调用方带鉴权令牌。'>
        <div className='form-grid two-columns'>
          <Field label='监听主机' hint='通常本机测试使用 0.0.0.0 或 127.0.0.1。'>
            <Input
              value={value.mock_bind_host}
              onChange={(next) => onChange({ ...value, mock_bind_host: next })}
              placeholder='0.0.0.0'
            />
          </Field>
          <Field label='监听端口' hint='内置 Mock 对外暴露的端口。环境内压测目标也会参考这里。'>
            <InputNumber
              value={value.mock_port}
              min={1}
              max={65535}
              onNumberChange={(next) => onChange({ ...value, mock_port: asNumber(next, value.mock_port) })}
            />
          </Field>
        </div>
        <div className='form-grid two-columns'>
          <Field label='鉴权模式' hint='如果启用，访问该 Mock 的客户端必须带正确令牌。适合多人共用环境时做隔离。'>
            <BooleanCard
              checked={value.mock_require_auth}
              label='要求调用方鉴权'
              description='校验 Authorization / x-api-key / api-key 中的令牌。'
              onChange={(checked) => onChange({ ...value, mock_require_auth: checked })}
            />
          </Field>
          <Field label='启动策略' hint='工作台启动后，会尝试自动恢复所有开启自动启动的环境监听器。'>
            <BooleanCard
              checked={value.auto_start}
              label='自动启动该环境'
              description='重启工作台后自动恢复这个环境的内置 Mock。'
              onChange={(checked) => onChange({ ...value, auto_start: checked })}
            />
          </Field>
        </div>
        <Field label='Mock 鉴权令牌' hint='仅在启用“要求调用方鉴权”时需要填写，建议使用容易识别的测试令牌。'>
          <Input
            value={value.mock_auth_token}
            onChange={(next) => onChange({ ...value, mock_auth_token: next })}
            placeholder='例如：workbench-token'
          />
        </Field>
      </SectionCard>

      <ActionBar onSave={onSave} onDelete={onDelete} deleteLabel='删除环境' />
    </div>
  );
}

interface MockProfileEditorProps {
  value: MockProfile;
  onChange: (value: MockProfile) => void;
  onSave: () => void;
}

export function MockProfileEditor(props: MockProfileEditorProps) {
  const { value, onChange, onSave } = props;
  const config = value.config;

  return (
    <div className='config-editor'>
      <EditorIntro
        title='Mock 行为配置'
        description='这里不是简单的固定响应，而是一整套随机行为模型。你可以控制延迟、错误率、文本生成特征、图片与视频任务模拟方式。'
        extra={<Tag color='green'>{config.models.length} 个模型</Tag>}
      />

      <SectionCard title='基础定义' description='先定义配置名称、可用模型列表，以及是否开放 pprof 调试端口。'>
        <div className='form-grid two-columns'>
          <Field label='配置名称' hint='建议按用途命名，例如“稳定回归模拟”“高抖动异常模拟”等。'>
            <Input value={value.name} onChange={(next) => onChange({ ...value, name: next })} placeholder='Mock 配置名称' />
          </Field>
          <Field label='调试端口' hint='当开启 pprof 后，Go 运行时分析信息会通过该端口暴露。'>
            <InputNumber
              value={config.pprof_port}
              min={1}
              max={65535}
              onNumberChange={(next) =>
                onChange({ ...value, config: { ...config, pprof_port: asNumber(next, config.pprof_port) } })
              }
            />
          </Field>
        </div>
        <Field label='pprof 调试' hint='仅用于性能分析和排查。平时关闭即可，需要分析 CPU/内存时再开启。'>
          <BooleanCard
            checked={config.enable_pprof}
            label='启用 pprof 分析端口'
            description='工作台会额外暴露运行时诊断信息，便于你做性能分析。'
            onChange={(checked) => onChange({ ...value, config: { ...config, enable_pprof: checked } })}
          />
        </Field>
        <StringListEditor
          label='可用模型列表'
          hint='这里的模型会出现在 `/v1/models` 结果里，也会作为聊天、图片、视频请求的默认候选集合。'
          values={config.models}
          addLabel='添加模型'
          placeholder='例如：gpt-4o-mini'
          onChange={(next) => onChange({ ...value, config: { ...config, models: next } })}
        />
      </SectionCard>

      <SectionCard title='全局随机策略' description='这一组参数决定所有模拟响应的延迟、错误注入比例，以及 token / chunk 的基础随机范围。'>
        <div className='form-grid two-columns'>
          <Field label='随机模式' hint='真随机适合压测波动场景；固定种子适合复现问题和做稳定回归。'>
            <Select
              value={config.random.mode}
              optionList={randomModeOptions.map((item) => ({ value: item.value, label: item.label }))}
              onChange={(next) =>
                onChange({
                  ...value,
                  config: { ...config, random: { ...config.random, mode: next as MockProfileConfig['random']['mode'] } }
                })
              }
            />
          </Field>
          <Field label='固定种子' hint='当随机模式为“固定种子”时，重复同样配置可以获得更稳定的模拟结果。'>
            <InputNumber
              value={config.random.seed}
              onNumberChange={(next) =>
                onChange({ ...value, config: { ...config, random: { ...config.random, seed: asNumber(next, config.random.seed) } } })
              }
            />
          </Field>
        </div>
        <div className='form-grid two-columns'>
          <IntRangeEditor
            label='响应延迟范围（毫秒）'
            hint='所有接口都会在这个范围内随机延迟，适合模拟真实上游抖动。'
            value={config.random.latency_ms}
            onChange={(next) => onChange({ ...value, config: { ...config, random: { ...config.random, latency_ms: next } } })}
          />
          <IntRangeEditor
            label='默认 token 数范围'
            hint='聊天、Responses 等文本接口在未特殊指定时，会按这里估算输出长度。'
            value={config.random.default_token_length}
            onChange={(next) =>
              onChange({ ...value, config: { ...config, random: { ...config.random, default_token_length: next } } })
            }
          />
        </div>
        <div className='form-grid two-columns'>
          <IntRangeEditor
            label='默认流式分块范围'
            hint='SSE 返回时会随机拆成多个片段，分块数越大越接近真实流式输出。'
            value={config.random.default_stream_chunks}
            onChange={(next) =>
              onChange({ ...value, config: { ...config, random: { ...config.random, default_stream_chunks: next } } })
            }
          />
          <div className='editor-subgrid'>
            <ProbabilityField
              label='普通错误率'
              hint='一般性请求错误的概率，适合模拟参数错误或非预期失败。'
              value={config.random.error_rate}
              onChange={(next) => onChange({ ...value, config: { ...config, random: { ...config.random, error_rate: next } } })}
            />
            <ProbabilityField
              label='限流错误率'
              hint='返回 429 的概率，用于模拟上游被打满时的速率限制行为。'
              value={config.random.too_many_requests_rate}
              onChange={(next) =>
                onChange({ ...value, config: { ...config, random: { ...config.random, too_many_requests_rate: next } } })
              }
            />
            <ProbabilityField
              label='服务端错误率'
              hint='返回 5xx 的概率，用于模拟上游内部故障或雪崩。'
              value={config.random.server_error_rate}
              onChange={(next) =>
                onChange({ ...value, config: { ...config, random: { ...config.random, server_error_rate: next } } })
              }
            />
            <ProbabilityField
              label='超时错误率'
              hint='用于模拟超时不返回、请求长时间挂起等情况。'
              value={config.random.timeout_rate}
              onChange={(next) =>
                onChange({ ...value, config: { ...config, random: { ...config.random, timeout_rate: next } } })
              }
            />
          </div>
        </div>
      </SectionCard>

      <SectionCard title='聊天补全行为' description='这里控制 `/v1/chat/completions` 的文本长度、是否允许流式输出、工具调用和最终回答的生成倾向。'>
        <TextBehaviorEditor
          value={config.chat}
          onChange={(next) => onChange({ ...value, config: { ...config, chat: next } })}
        />
      </SectionCard>

      <SectionCard title='Responses 行为' description='这里控制 `/v1/responses` 的输出风格。你可以把它配置得更偏工具化，也可以更偏纯文本。'>
        <TextBehaviorEditor
          value={config.responses}
          onChange={(next) => onChange({ ...value, config: { ...config, responses: next } })}
        />
      </SectionCard>

      <SectionCard title='图片生成行为' description='这里配置图片尺寸、返回 URL 或 Base64 的比例、水印策略和图片体积范围。'>
        <div className='form-grid two-columns'>
          <IntRangeEditor
            label='单次图片数量范围'
            hint='同一次请求返回几张图片。适合模拟不同上游的多图输出能力。'
            value={config.images.image_count}
            onChange={(next) => onChange({ ...value, config: { ...config, images: { ...config.images, image_count: next } } })}
          />
          <IntRangeEditor
            label='图片字节大小范围'
            hint='生成出来的图片体积范围。数值越大，下载和解析成本越高。'
            value={config.images.image_bytes}
            onChange={(next) => onChange({ ...value, config: { ...config, images: { ...config.images, image_bytes: next } } })}
          />
        </div>
        <div className='form-grid two-columns'>
          <ProbabilityField
            label='URL 返回比例'
            hint='越接近 1，越倾向返回图片 URL；越接近 0，越倾向返回 Base64。'
            value={config.images.response_url_rate}
            onChange={(next) =>
              onChange({ ...value, config: { ...config, images: { ...config.images, response_url_rate: next } } })
            }
          />
          <ProbabilityField
            label='水印概率'
            hint='控制模拟图片中附带调试水印的概率，适合验证客户端对测试素材的识别。'
            value={config.images.watermark_rate}
            onChange={(next) => onChange({ ...value, config: { ...config, images: { ...config.images, watermark_rate: next } } })}
          />
        </div>
        <StringListEditor
          label='支持尺寸列表'
          hint='这些尺寸会作为图片生成接口的可选尺寸池。'
          values={config.images.sizes}
          addLabel='添加尺寸'
          placeholder='例如：1024x1024'
          onChange={(next) => onChange({ ...value, config: { ...config, images: { ...config.images, sizes: next } } })}
        />
        <StringListEditor
          label='水印文本列表'
          hint='当命中水印概率时，会随机从这里取一个文本绘制在图片上。'
          values={config.images.watermark_texts}
          addLabel='添加水印文本'
          placeholder='例如：simulated'
          onChange={(next) =>
            onChange({ ...value, config: { ...config, images: { ...config.images, watermark_texts: next } } })
          }
        />
        <StringListEditor
          label='背景色板'
          hint='图片背景会从这里随机选色。适合模拟更丰富的视觉输出分布。'
          values={config.images.background_palette}
          addLabel='添加色值'
          placeholder='例如：#0F172A'
          onChange={(next) =>
            onChange({ ...value, config: { ...config, images: { ...config.images, background_palette: next } } })
          }
        />
      </SectionCard>

      <SectionCard title='视频任务行为' description='这里模拟异步视频生成任务的时长、轮询速度、失败率与产物大小，用于验证任务流编排。'>
        <div className='form-grid two-columns'>
          <FloatRangeEditor
            label='视频时长范围（秒）'
            hint='创建任务时会在这个范围内随机生成时长。'
            value={config.videos.durations_seconds}
            onChange={(next) =>
              onChange({ ...value, config: { ...config, videos: { ...config.videos, durations_seconds: next } } })
            }
          />
          <IntRangeEditor
            label='FPS 范围'
            hint='控制视频帧率的随机区间。'
            value={config.videos.fps}
            onChange={(next) => onChange({ ...value, config: { ...config, videos: { ...config.videos, fps: next } } })}
          />
        </div>
        <div className='form-grid two-columns'>
          <IntRangeEditor
            label='轮询间隔范围（毫秒）'
            hint='任务状态变化的节奏与这里的随机区间有关。'
            value={config.videos.poll_interval_ms}
            onChange={(next) =>
              onChange({ ...value, config: { ...config, videos: { ...config.videos, poll_interval_ms: next } } })
            }
          />
          <IntRangeEditor
            label='视频字节大小范围'
            hint='生成完成后的视频文件体积范围。'
            value={config.videos.video_bytes}
            onChange={(next) =>
              onChange({ ...value, config: { ...config, videos: { ...config.videos, video_bytes: next } } })
            }
          />
        </div>
        <div className='form-grid two-columns'>
          <IntRangeEditor
            label='进度抖动范围'
            hint='轮询返回的进度会加入这里的抖动，避免每次都线性增长。'
            value={config.videos.progress_jitter}
            onChange={(next) =>
              onChange({ ...value, config: { ...config, videos: { ...config.videos, progress_jitter: next } } })
            }
          />
          <ProbabilityField
            label='视频失败率'
            hint='控制视频任务在运行过程中失败的概率。'
            value={config.videos.failure_rate}
            onChange={(next) => onChange({ ...value, config: { ...config, videos: { ...config.videos, failure_rate: next } } })}
          />
        </div>
        <StringListEditor
          label='分辨率列表'
          hint='视频任务会从这里随机选择尺寸，例如 640x360、1280x720。'
          values={config.videos.resolutions}
          addLabel='添加分辨率'
          placeholder='例如：1280x720'
          onChange={(next) => onChange({ ...value, config: { ...config, videos: { ...config.videos, resolutions: next } } })}
        />
      </SectionCard>

      <ActionBar onSave={onSave} />
    </div>
  );
}

interface RunProfileEditorProps {
  value: RunProfile;
  onChange: (value: RunProfile) => void;
  onSave: () => void;
}

export function RunProfileEditor(props: RunProfileEditorProps) {
  const { value, onChange, onSave } = props;
  const config = value.config;

  return (
    <div className='config-editor'>
      <EditorIntro
        title='压测参数配置'
        description='这里控制并发、升压时间、持续时间、请求超时和采样策略。你可以用它定义一套“平稳巡航”或“强压打满”的压测模板。'
        extra={<Tag color='orange'>并发 {config.total_concurrency}</Tag>}
      />

      <SectionCard title='基础压测参数' description='核心压测节奏：并发、升压、持续时长、请求上限和单请求超时。'>
        <div className='form-grid two-columns'>
          <Field label='配置名称' hint='建议按目标和压力等级命名，例如“中压 SSE 回归”“图片接口突刺压测”。'>
            <Input value={value.name} onChange={(next) => onChange({ ...value, name: next })} placeholder='压测配置名称' />
          </Field>
          <Field label='总并发数' hint='所有场景共享这个并发上限，系统会按场景权重拆分请求。'>
            <InputNumber
              value={config.total_concurrency}
              min={1}
              onNumberChange={(next) =>
                onChange({ ...value, config: { ...config, total_concurrency: asNumber(next, config.total_concurrency) } })
              }
            />
          </Field>
          <Field label='升压时间（秒）' hint='从 0 增长到目标并发所花的时间，避免一开始瞬间把目标打满。'>
            <InputNumber
              value={config.ramp_up_sec}
              min={0}
              onNumberChange={(next) =>
                onChange({ ...value, config: { ...config, ramp_up_sec: asNumber(next, config.ramp_up_sec) } })
              }
            />
          </Field>
          <Field label='持续时间（秒）' hint='压测正式保持稳定输出的总时长。'>
            <InputNumber
              value={config.duration_sec}
              min={1}
              onNumberChange={(next) =>
                onChange({ ...value, config: { ...config, duration_sec: asNumber(next, config.duration_sec) } })
              }
            />
          </Field>
          <Field label='请求总数上限' hint='填 0 表示不限制，只按持续时间结束；填正数表示累计到指定请求数后提前结束。'>
            <InputNumber
              value={config.max_requests}
              min={0}
              onNumberChange={(next) =>
                onChange({ ...value, config: { ...config, max_requests: asNumber(next, config.max_requests) } })
              }
            />
          </Field>
          <Field label='请求超时（毫秒）' hint='超过该时间未完成的请求会记为超时错误。'>
            <InputNumber
              value={config.request_timeout_ms}
              min={100}
              onNumberChange={(next) =>
                onChange({ ...value, config: { ...config, request_timeout_ms: asNumber(next, config.request_timeout_ms) } })
              }
            />
          </Field>
        </div>
      </SectionCard>

      <SectionCard title='采样与脱敏' description='工作台会把一部分请求、错误和响应内容落盘，方便回看。这里控制样本数量和敏感头脱敏策略。'>
        <div className='form-grid two-columns'>
          <Field label='成功请求样本上限' hint='最多保留多少条普通请求样本，便于回放和复盘。'>
            <InputNumber
              value={config.sampling.max_request_samples}
              min={1}
              onNumberChange={(next) =>
                onChange({
                  ...value,
                  config: {
                    ...config,
                    sampling: { ...config.sampling, max_request_samples: asNumber(next, config.sampling.max_request_samples) }
                  }
                })
              }
            />
          </Field>
          <Field label='错误样本上限' hint='错误样本一般价值更高，建议保留得略多一些。'>
            <InputNumber
              value={config.sampling.max_error_samples}
              min={1}
              onNumberChange={(next) =>
                onChange({
                  ...value,
                  config: {
                    ...config,
                    sampling: { ...config.sampling, max_error_samples: asNumber(next, config.sampling.max_error_samples) }
                  }
                })
              }
            />
          </Field>
          <Field label='响应体最大保留字节' hint='超过这个阈值的响应体会被截断，以避免样本库膨胀过快。'>
            <InputNumber
              value={config.sampling.max_body_bytes}
              min={256}
              onNumberChange={(next) =>
                onChange({
                  ...value,
                  config: {
                    ...config,
                    sampling: { ...config.sampling, max_body_bytes: asNumber(next, config.sampling.max_body_bytes) }
                  }
                })
              }
            />
          </Field>
        </div>
        <StringListEditor
          label='脱敏请求头'
          hint='这些请求头在样本记录中会自动做脱敏处理，建议把所有鉴权和密钥头都放进来。'
          values={config.sampling.mask_headers}
          addLabel='添加脱敏头'
          placeholder='例如：authorization'
          onChange={(next) =>
            onChange({ ...value, config: { ...config, sampling: { ...config.sampling, mask_headers: next } } })
          }
        />
      </SectionCard>

      <ActionBar onSave={onSave} />
    </div>
  );
}

interface ScenarioEditorProps {
  value: Scenario;
  onChange: (value: Scenario) => void;
  onSave: () => void;
}

export function ScenarioEditor(props: ScenarioEditorProps) {
  const { value, onChange, onSave } = props;
  const config = value.config;

  const updateConfig = (next: ScenarioConfig) => {
    onChange({ ...value, name: next.name, config: next });
  };

  const isTaskFlow = config.mode === 'task_flow';

  return (
    <div className='config-editor'>
      <EditorIntro
        title='场景配置'
        description='场景定义的是“发什么请求、怎么判断成功、是否需要轮询任务结果”。你可以用它覆盖聊天、图片、视频或任意 HTTP 接口。'
        extra={<Tag color='purple'>{formatScenarioMode(config.mode)}</Tag>}
      />

      <SectionCard title='场景基本信息' description='先定义显示名称、内部标识、是否启用、权重、模式和预置模板。权重越高，该场景在压测中的流量占比越大。'>
        <div className='form-grid two-columns'>
          <Field label='场景名称' hint='用于运行历史、图表拆分和统计表展示。建议写成业务可读的名称。'>
            <Input value={config.name} onChange={(next) => updateConfig({ ...config, name: next })} placeholder='例如：聊天流式对话' />
          </Field>
          <Field label='场景标识' hint='用于内部统计和样本记录归类。已创建后的修改不会改变数据库主键，但会影响后续统计标签。'>
            <Input value={config.id} onChange={(next) => updateConfig({ ...config, id: next })} placeholder='例如：chat-sse-main' />
          </Field>
          <Field label='预置模板' hint='选择后会自动把请求方法、路径、请求体和任务流模板切到对应的推荐值。'>
            <Select
              value={config.preset || 'raw_http'}
              optionList={presetOptions.map((item) => ({ value: item.value, label: item.label }))}
              onChange={(next) => updateConfig(applyScenarioPreset(config, String(next)))}
            />
          </Field>
          <Field label='执行模式' hint='单次请求适合普通 API，流式 SSE 适合聊天输出，任务轮询适合视频等异步接口。'>
            <Select
              value={config.mode}
              optionList={scenarioModeOptions.map((item) => ({ value: item.value, label: item.label }))}
              onChange={(next) =>
                updateConfig({
                  ...config,
                  mode: next as ScenarioConfig['mode']
                })
              }
            />
          </Field>
          <Field label='场景权重' hint='不同场景会按权重分配请求量，例如 5:1 表示一个场景的流量大约是另一个的五倍。'>
            <InputNumber
              value={config.weight}
              min={1}
              onNumberChange={(next) => updateConfig({ ...config, weight: asNumber(next, config.weight) })}
            />
          </Field>
          <Field label='启用状态' hint='关闭后会保留配置，但不会参与压测流量分配。'>
            <BooleanCard
              checked={config.enabled}
              label='启用该场景'
              description='停用的场景仍然会保存在项目里，方便之后重新启用。'
              onChange={(checked) => updateConfig({ ...config, enabled: checked })}
            />
          </Field>
        </div>
      </SectionCard>

      {!isTaskFlow ? (
        <SectionCard title='请求定义' description='配置单次请求或流式 SSE 的方法、路径、请求头、请求体和预期状态码。'>
          <RequestConfigEditor
            title='请求参数'
            description='这里定义真实发出的 HTTP 请求内容。对于聊天和图片接口，通常请求体是 JSON；你也可以写纯文本或其他格式。'
            value={{
              method: config.method,
              path: config.path,
              headers: config.headers,
              body: config.body,
              expected_statuses: config.expected_statuses
            }}
            pathLabel='请求路径'
            pathHint='可以填写绝对 URL，也可以填写以 / 开头的相对路径。'
            onChange={(next) =>
              updateConfig({
                ...config,
                method: next.method,
                path: next.path,
                headers: next.headers,
                body: next.body,
                expected_statuses: next.expected_statuses
              })
            }
          />
        </SectionCard>
      ) : (
        <>
          <SectionCard title='任务提交流程' description='第一步先提交异步任务，例如创建视频生成任务。这里定义创建任务时发送的请求。'>
            <RequestConfigEditor
              title='提交任务请求'
              description='提交请求通常返回任务 ID，后续轮询会从这个任务 ID 开始。'
              value={config.task_flow.submit_request}
              pathLabel='提交路径'
              pathHint='例如 /v1/videos，用于创建异步任务。'
              onChange={(next) =>
                updateConfig({
                  ...config,
                  task_flow: { ...config.task_flow, submit_request: next }
                })
              }
            />
          </SectionCard>

          <SectionCard title='任务轮询流程' description='第二步轮询任务状态，直到命中成功状态或失败状态。这里定义轮询地址、提取器和轮询终止条件。'>
            <div className='form-grid two-columns'>
              <Field label='任务 ID 提取路径' hint='从提交任务接口的 JSON 响应中提取任务 ID，例如 id。'>
                <Input
                  value={config.extractors.task_id_path}
                  onChange={(next) => updateConfig({ ...config, extractors: { ...config.extractors, task_id_path: next } })}
                  placeholder='例如：id'
                />
              </Field>
              <Field label='任务状态提取路径' hint='从轮询接口响应中提取任务状态，例如 status。'>
                <Input
                  value={config.extractors.task_status_path}
                  onChange={(next) =>
                    updateConfig({ ...config, extractors: { ...config.extractors, task_status_path: next } })
                  }
                  placeholder='例如：status'
                />
              </Field>
              <Field label='轮询间隔（毫秒）' hint='每次轮询之间等待多久，越小越快，但会增加轮询压力。'>
                <InputNumber
                  value={config.task_flow.poll_interval_ms}
                  min={100}
                  onNumberChange={(next) =>
                    updateConfig({
                      ...config,
                      task_flow: { ...config.task_flow, poll_interval_ms: asNumber(next, config.task_flow.poll_interval_ms) }
                    })
                  }
                />
              </Field>
              <Field label='最大轮询次数' hint='超过上限仍未成功或失败时，会把场景视为异常。'>
                <InputNumber
                  value={config.task_flow.max_polls}
                  min={1}
                  onNumberChange={(next) =>
                    updateConfig({
                      ...config,
                      task_flow: { ...config.task_flow, max_polls: asNumber(next, config.task_flow.max_polls) }
                    })
                  }
                />
              </Field>
            </div>
            <RequestConfigEditor
              title='轮询请求'
              description='路径模板中必须带 `{task_id}` 占位符，工作台会用提取到的任务 ID 替换。'
              value={{
                method: config.task_flow.poll_request.method,
                path: config.task_flow.poll_request.path_template,
                headers: config.task_flow.poll_request.headers,
                body: '',
                expected_statuses: config.task_flow.poll_request.expected_statuses
              }}
              bodyDisabled
              pathLabel='轮询路径模板'
              pathHint='例如 /v1/videos/{task_id}，会自动替换成真实任务 ID。'
              onChange={(next) =>
                updateConfig({
                  ...config,
                  task_flow: {
                    ...config.task_flow,
                    poll_request: {
                      ...config.task_flow.poll_request,
                      method: next.method,
                      path_template: next.path,
                      headers: next.headers,
                      expected_statuses: next.expected_statuses
                    }
                  }
                })
              }
            />
            <div className='form-grid two-columns'>
              <StringListEditor
                label='成功状态值'
                hint='轮询接口响应中的状态命中这些值之一时，判定任务成功完成。'
                values={config.task_flow.success_values}
                addLabel='添加成功状态'
                placeholder='例如：completed'
                onChange={(next) =>
                  updateConfig({ ...config, task_flow: { ...config.task_flow, success_values: next } })
                }
              />
              <StringListEditor
                label='失败状态值'
                hint='轮询接口响应中的状态命中这些值之一时，判定任务失败。'
                values={config.task_flow.failure_values}
                addLabel='添加失败状态'
                placeholder='例如：failed'
                onChange={(next) =>
                  updateConfig({ ...config, task_flow: { ...config.task_flow, failure_values: next } })
                }
              />
            </div>
          </SectionCard>
        </>
      )}

      <ActionBar onSave={onSave} />
    </div>
  );
}

function TextBehaviorEditor(props: {
  value: TextBehaviorConfig;
  onChange: (value: TextBehaviorConfig) => void;
}) {
  const { value, onChange } = props;

  return (
    <div className='section-stack'>
      <div className='form-grid two-columns'>
        <IntRangeEditor
          label='文本 token 范围'
          hint='决定生成文本的长度区间。范围越大，返回内容波动越明显。'
          value={value.text_tokens}
          onChange={(next) => onChange({ ...value, text_tokens: next })}
        />
        <BooleanCard
          checked={value.allow_stream}
          label='允许流式输出'
          description='关闭后，即使客户端请求流式，模拟器也只返回完整结果。'
          onChange={(checked) => onChange({ ...value, allow_stream: checked })}
        />
      </div>
      <div className='form-grid two-columns'>
        <IntRangeEditor
          label='工具调用次数范围'
          hint='命中工具调用后，一次响应里可能出现的工具调用数量。'
          value={value.tool_call_count}
          onChange={(next) => onChange({ ...value, tool_call_count: next })}
        />
        <IntRangeEditor
          label='工具参数字节范围'
          hint='用于模拟工具参数 payload 的体积，方便压测大参数场景。'
          value={value.tool_arguments_bytes}
          onChange={(next) => onChange({ ...value, tool_arguments_bytes: next })}
        />
      </div>
      <div className='editor-subgrid'>
        <ProbabilityField
          label='用量统计出现概率'
          hint='控制响应中是否附带 usage 字段。'
          value={value.usage_probability}
          onChange={(next) => onChange({ ...value, usage_probability: next })}
        />
        <ProbabilityField
          label='工具调用概率'
          hint='控制模型回答中出现 tool_calls 的概率。'
          value={value.tool_call_probability}
          onChange={(next) => onChange({ ...value, tool_call_probability: next })}
        />
        <ProbabilityField
          label='MCP 工具概率'
          hint='在工具调用里进一步控制是否返回 MCP 风格工具。'
          value={value.mcp_tool_probability}
          onChange={(next) => onChange({ ...value, mcp_tool_probability: next })}
        />
        <ProbabilityField
          label='最终回答概率'
          hint='命中工具调用后，模型是否会继续输出最终回答。'
          value={value.final_answer_probability}
          onChange={(next) => onChange({ ...value, final_answer_probability: next })}
        />
        <ProbabilityField
          label='System Fingerprint 概率'
          hint='控制是否在响应里附带 system_fingerprint。'
          value={value.system_fingerprint_probability}
          onChange={(next) => onChange({ ...value, system_fingerprint_probability: next })}
        />
      </div>
    </div>
  );
}

function RequestConfigEditor(props: {
  title: string;
  description: string;
  value: ScenarioRequestConfig | { method: string; path: string; headers: Record<string, string>; body: string; expected_statuses: number[] };
  onChange: (value: { method: string; path: string; headers: Record<string, string>; body: string; expected_statuses: number[] }) => void;
  pathLabel?: string;
  pathHint?: string;
  bodyDisabled?: boolean;
}) {
  const { title, description, value, onChange, pathLabel = '请求路径', pathHint, bodyDisabled } = props;

  return (
    <div className='request-editor'>
      <div className='request-editor-head'>
        <Typography.Title heading={6}>{title}</Typography.Title>
        <Typography.Text type='tertiary'>{description}</Typography.Text>
      </div>
      <div className='form-grid two-columns'>
        <Field label='请求方法' hint='支持常见 HTTP 方法，通常聊天与图片接口使用 POST。'>
          <Select
            value={value.method}
            optionList={httpMethodOptions.map((item) => ({ value: item, label: item }))}
            onChange={(next) => onChange({ ...value, method: String(next) })}
          />
        </Field>
        <Field label={pathLabel} hint={pathHint ?? '建议填写以 / 开头的路径，例如 /v1/chat/completions。'}>
          <Input value={value.path} onChange={(next) => onChange({ ...value, path: next })} placeholder='/v1/models' />
        </Field>
      </div>
      <StatusCodeEditor
        label='期望状态码'
        hint='只要返回状态码命中这里任意一个值，就会视为协议层成功。'
        values={value.expected_statuses}
        onChange={(next) => onChange({ ...value, expected_statuses: next })}
      />
      <KeyValueEditor
        label='请求头'
        hint='只对当前场景或当前阶段生效，会叠加在环境默认请求头之上。'
        value={value.headers}
        keyPlaceholder='请求头名称'
        valuePlaceholder='请求头内容'
        onChange={(next) => onChange({ ...value, headers: next })}
      />
      <Field label='请求体' hint='这里填写真实发给目标接口的原始请求体。通常是 JSON，也可以是纯文本或其他格式。'>
        <TextArea
          rows={8}
          value={value.body}
          disabled={bodyDisabled}
          placeholder={bodyDisabled ? '轮询请求通常不需要请求体。' : '请输入请求体，例如 JSON。'}
          onChange={(next) => onChange({ ...value, body: next })}
        />
      </Field>
    </div>
  );
}

function KeyValueEditor(props: {
  label: string;
  hint: string;
  value: Record<string, string>;
  keyPlaceholder: string;
  valuePlaceholder: string;
  onChange: (value: Record<string, string>) => void;
}) {
  const rows = objectToRows(props.value);

  const updateRow = (index: number, patch: Partial<{ key: string; value: string }>) => {
    const next = rows.map((row, rowIndex) => (rowIndex === index ? { ...row, ...patch } : row));
    props.onChange(rowsToObject(next));
  };

  const removeRow = (index: number) => {
    const next = rows.filter((_, rowIndex) => rowIndex !== index);
    props.onChange(rowsToObject(next));
  };

  const addRow = () => {
    props.onChange(rowsToObject([...rows, { key: '', value: '' }]));
  };

  return (
    <Field label={props.label} hint={props.hint}>
      <div className='list-editor'>
        {rows.map((row, index) => (
          <div className='kv-row' key={`${props.label}-${index}`}>
            <Input
              value={row.key}
              placeholder={props.keyPlaceholder}
              onChange={(next) => updateRow(index, { key: next })}
            />
            <Input
              value={row.value}
              placeholder={props.valuePlaceholder}
              onChange={(next) => updateRow(index, { value: next })}
            />
            <Button type='danger' theme='borderless' onClick={() => removeRow(index)}>
              删除
            </Button>
          </div>
        ))}
        <Button theme='light' onClick={addRow}>
          添加一行
        </Button>
      </div>
    </Field>
  );
}

function StringListEditor(props: {
  label: string;
  hint: string;
  values: string[];
  addLabel: string;
  placeholder: string;
  onChange: (values: string[]) => void;
}) {
  const rows = props.values.length > 0 ? props.values : [''];

  const updateValue = (index: number, nextValue: string) => {
    const next = rows.map((item, rowIndex) => (rowIndex === index ? nextValue : item));
    props.onChange(next.filter((item) => item.trim().length > 0 || next.length === 1));
  };

  const removeValue = (index: number) => {
    const next = rows.filter((_, rowIndex) => rowIndex !== index);
    props.onChange(next);
  };

  const addValue = () => props.onChange([...props.values, '']);

  return (
    <Field label={props.label} hint={props.hint}>
      <div className='list-editor'>
        {rows.map((item, index) => (
          <div className='single-row' key={`${props.label}-${index}`}>
            <Input value={item} placeholder={props.placeholder} onChange={(next) => updateValue(index, next)} />
            <Button type='danger' theme='borderless' onClick={() => removeValue(index)}>
              删除
            </Button>
          </div>
        ))}
        <Button theme='light' onClick={addValue}>
          {props.addLabel}
        </Button>
      </div>
    </Field>
  );
}

function StatusCodeEditor(props: {
  label: string;
  hint: string;
  values: number[];
  onChange: (values: number[]) => void;
}) {
  const rows = props.values.length > 0 ? props.values : [200];

  const updateValue = (index: number, nextValue: number) => {
    const next = rows.map((item, rowIndex) => (rowIndex === index ? nextValue : item));
    props.onChange(next.filter((item) => Number.isFinite(item)));
  };

  const removeValue = (index: number) => {
    props.onChange(rows.filter((_, rowIndex) => rowIndex !== index));
  };

  return (
    <Field label={props.label} hint={props.hint}>
      <div className='list-editor'>
        {rows.map((item, index) => (
          <div className='single-row' key={`${props.label}-${index}`}>
            <InputNumber value={item} min={100} max={599} onNumberChange={(next) => updateValue(index, asNumber(next, item))} />
            <Button type='danger' theme='borderless' onClick={() => removeValue(index)}>
              删除
            </Button>
          </div>
        ))}
        <Button theme='light' onClick={() => props.onChange([...props.values, 200])}>
          添加状态码
        </Button>
      </div>
    </Field>
  );
}

function IntRangeEditor(props: {
  label: string;
  hint: string;
  value: IntRange;
  onChange: (value: IntRange) => void;
}) {
  return (
    <Field label={props.label} hint={props.hint}>
      <div className='range-grid'>
        <Field label='最小值'>
          <InputNumber value={props.value.min} onNumberChange={(next) => props.onChange({ ...props.value, min: asNumber(next, props.value.min) })} />
        </Field>
        <Field label='最大值'>
          <InputNumber value={props.value.max} onNumberChange={(next) => props.onChange({ ...props.value, max: asNumber(next, props.value.max) })} />
        </Field>
      </div>
    </Field>
  );
}

function FloatRangeEditor(props: {
  label: string;
  hint: string;
  value: FloatRange;
  onChange: (value: FloatRange) => void;
}) {
  return (
    <Field label={props.label} hint={props.hint}>
      <div className='range-grid'>
        <Field label='最小值'>
          <InputNumber
            value={props.value.min}
            step={0.1}
            onNumberChange={(next) => props.onChange({ ...props.value, min: asNumber(next, props.value.min) })}
          />
        </Field>
        <Field label='最大值'>
          <InputNumber
            value={props.value.max}
            step={0.1}
            onNumberChange={(next) => props.onChange({ ...props.value, max: asNumber(next, props.value.max) })}
          />
        </Field>
      </div>
    </Field>
  );
}

function ProbabilityField(props: {
  label: string;
  hint: string;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <Field label={props.label} hint={props.hint}>
      <InputNumber value={props.value} min={0} max={1} step={0.01} onNumberChange={(next) => props.onChange(asNumber(next, props.value))} />
    </Field>
  );
}

function BooleanCard(props: {
  checked: boolean;
  label: string;
  description: string;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className='boolean-card'>
      <div className='boolean-copy'>
        <Typography.Text strong>{props.label}</Typography.Text>
        <Typography.Text type='tertiary'>{props.description}</Typography.Text>
      </div>
      <Switch checked={props.checked} onChange={props.onChange} />
    </div>
  );
}

function EditorIntro(props: { title: string; description: string; extra?: ReactNode }) {
  return (
    <div className='editor-intro'>
      <div>
        <Typography.Title heading={5}>{props.title}</Typography.Title>
        <Typography.Text type='tertiary'>{props.description}</Typography.Text>
      </div>
      {props.extra}
    </div>
  );
}

function SectionCard(props: { title: string; description: string; children: ReactNode }) {
  return (
    <section className='editor-section'>
      <div className='editor-section-head'>
        <Typography.Title heading={6}>{props.title}</Typography.Title>
        <Typography.Text type='tertiary'>{props.description}</Typography.Text>
      </div>
      <div className='section-stack'>{props.children}</div>
    </section>
  );
}

function Field(props: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className='field-block'>
      <Typography.Text strong>{props.label}</Typography.Text>
      {props.hint ? <Typography.Text type='tertiary'>{props.hint}</Typography.Text> : null}
      {props.children}
    </div>
  );
}

function ActionBar(props: { onSave: () => void; onDelete?: () => void; deleteLabel?: string }) {
  return (
    <div className='editor-actions'>
      <Button theme='solid' type='primary' onClick={props.onSave}>
        保存配置
      </Button>
      {props.onDelete ? (
        <Button type='danger' onClick={props.onDelete}>
          {props.deleteLabel ?? '删除'}
        </Button>
      ) : null}
    </div>
  );
}

function objectToRows(value: Record<string, string>) {
  const entries = Object.entries(value);
  if (entries.length === 0) {
    return [{ key: '', value: '' }];
  }
  return entries.map(([key, itemValue]) => ({ key, value: itemValue }));
}

function rowsToObject(rows: Array<{ key: string; value: string }>) {
  return rows.reduce<Record<string, string>>((accumulator, item) => {
    const key = item.key.trim();
    if (key) {
      accumulator[key] = item.value;
    }
    return accumulator;
  }, {});
}

function asNumber(value: string | number | undefined, fallback: number) {
  const next = Number(value);
  return Number.isFinite(next) ? next : fallback;
}
