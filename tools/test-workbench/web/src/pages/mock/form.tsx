import { Input, InputNumber, Select, Tag } from '@douyinfe/semi-ui';
import {
  ActionBar,
  BooleanCard,
  EditorIntro,
  Field,
  FloatRangeEditor,
  IntRangeEditor,
  ProbabilityField,
  SectionCard,
  StringListEditor,
  asNumber
} from '../../components/form-kit';
import type { MockProfile, MockProfileConfig, TextBehaviorConfig } from '../../types';

const randomModeOptions = [
  { value: 'true_random', label: '真随机' },
  { value: 'seeded', label: '固定种子' }
] as const;

interface MockProfileFormProps {
  value: MockProfile;
  onChange: (value: MockProfile) => void;
  onSave: () => void;
}

export function MockProfileForm(props: MockProfileFormProps) {
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
        <TextBehaviorSection value={config.chat} onChange={(next) => onChange({ ...value, config: { ...config, chat: next } })} />
      </SectionCard>

      <SectionCard title='Responses 行为' description='这里控制 `/v1/responses` 的输出风格。你可以把它配置得更偏工具化，也可以更偏纯文本。'>
        <TextBehaviorSection value={config.responses} onChange={(next) => onChange({ ...value, config: { ...config, responses: next } })} />
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

function TextBehaviorSection(props: {
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
