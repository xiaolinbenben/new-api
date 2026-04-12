import { Input, InputNumber, Select, Tag } from '@douyinfe/semi-ui';
import type { Environment, MockProfile, RunProfile } from '../../types';
import { formatTargetType } from '../../features/workbench/helpers';
import { ActionBar, BooleanCard, EditorIntro, Field, KeyValueEditor, SectionCard, asNumber } from '../../components/form-kit';

const targetTypeOptions = [
  { value: 'internal_mock', label: '内置 Mock（internal_mock）' },
  { value: 'external_http', label: '外部 HTTP（external_http）' }
] as const;

interface EnvironmentFormProps {
  value: Environment;
  mockProfiles: MockProfile[];
  runProfiles: RunProfile[];
  onChange: (value: Environment) => void;
  onSave: () => void;
  onDelete?: () => void;
}

export function EnvironmentForm(props: EnvironmentFormProps) {
  const { value, mockProfiles, runProfiles, onChange, onSave, onDelete } = props;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <EditorIntro
        title='环境配置'
        description='这里定义压测目标、默认请求头、内置 Mock 监听器以及本环境绑定的默认配置。切换不同环境后，工作台会自动基于这里的设置启动或执行。'
        extra={<Tag color='light-blue'>{formatTargetType(value.target_type)}</Tag>}
      />

      <SectionCard title='基础信息' description='先定义环境本身的名称、目标类型，以及本环境默认绑定哪一套 Mock / 压测配置。'>
        <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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
        <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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
        <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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
        <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
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
