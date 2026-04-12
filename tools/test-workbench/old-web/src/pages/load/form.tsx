import { Input, InputNumber, Select, Tag } from '@douyinfe/semi-ui';
import {
  ActionBar,
  BooleanCard,
  EditorIntro,
  Field,
  IntRangeEditor,
  KeyValueEditor,
  ProbabilityField,
  RequestConfigEditor,
  SectionCard,
  StringListEditor,
  asNumber
} from '../../components/form-kit';
import { applyScenarioPreset, formatScenarioMode } from '../../features/workbench/helpers';
import type { RunProfile, Scenario, ScenarioConfig, TextBehaviorConfig } from '../../types';

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

interface RunProfileFormProps {
  value: RunProfile;
  onChange: (value: RunProfile) => void;
  onSave: () => void;
}

export function RunProfileForm(props: RunProfileFormProps) {
  const { value, onChange, onSave } = props;
  const config = value.config;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <EditorIntro
        title='压测参数配置'
        description='这里控制并发、升压时间、持续时间、请求超时和采样策略。你可以用它定义一套“平稳巡航”或“强压打满”的压测模板。'
        extra={<Tag color='orange'>并发 {config.total_concurrency}</Tag>}
      />

      <SectionCard title='基础压测参数' description='核心压测节奏：并发、升压、持续时长、请求上限和单请求超时。'>
        <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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
        <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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

interface ScenarioFormProps {
  value: Scenario;
  onChange: (value: Scenario) => void;
  onSave: () => void;
}

export function ScenarioForm(props: ScenarioFormProps) {
  const { value, onChange, onSave } = props;
  const config = value.config;

  const updateConfig = (next: ScenarioConfig) => {
    onChange({ ...value, name: next.name, config: next });
  };

  const isTaskFlow = config.mode === 'task_flow';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <EditorIntro
        title='场景配置'
        description='场景定义的是“发什么请求、怎么判断成功、是否需要轮询任务结果”。你可以用它覆盖聊天、图片、视频或任意 HTTP 接口。'
        extra={<Tag color='purple'>{formatScenarioMode(config.mode)}</Tag>}
      />

      <SectionCard title='场景基本信息' description='先定义显示名称、内部标识、是否启用、权重、模式和预置模板。权重越高，该场景在压测中的流量占比越大。'>
        <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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
              onChange={(next) => updateConfig({ ...config, mode: next as ScenarioConfig['mode'] })}
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
            <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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
                  onChange={(next) => updateConfig({ ...config, extractors: { ...config.extractors, task_status_path: next } })}
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
            <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
              <StringListEditor
                label='成功状态值'
                hint='轮询接口响应中的状态命中这些值之一时，判定任务成功完成。'
                values={config.task_flow.success_values}
                addLabel='添加成功状态'
                placeholder='例如：completed'
                onChange={(next) => updateConfig({ ...config, task_flow: { ...config.task_flow, success_values: next } })}
              />
              <StringListEditor
                label='失败状态值'
                hint='轮询接口响应中的状态命中这些值之一时，判定任务失败。'
                values={config.task_flow.failure_values}
                addLabel='添加失败状态'
                placeholder='例如：failed'
                onChange={(next) => updateConfig({ ...config, task_flow: { ...config.task_flow, failure_values: next } })}
              />
            </div>
          </SectionCard>
        </>
      )}

      <ActionBar onSave={onSave} />
    </div>
  );
}
