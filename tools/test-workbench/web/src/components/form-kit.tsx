import type { ReactNode } from 'react';
import { Button, Input, InputNumber, Select, Switch, TextArea, Typography } from '@douyinfe/semi-ui';
import type { FloatRange, IntRange } from '../types';

const httpMethodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD'];

export interface RequestFormValue {
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  expected_statuses: number[];
}

export function EditorIntro(props: { title: string; description: string; extra?: ReactNode }) {
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

export function SectionCard(props: { title: string; description: string; children: ReactNode }) {
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

export function Field(props: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className='field-block'>
      <Typography.Text strong>{props.label}</Typography.Text>
      {props.hint ? <Typography.Text type='tertiary'>{props.hint}</Typography.Text> : null}
      {props.children}
    </div>
  );
}

export function ActionBar(props: { onSave: () => void; onDelete?: () => void; deleteLabel?: string }) {
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

export function KeyValueEditor(props: {
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

export function StringListEditor(props: {
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

export function StatusCodeEditor(props: {
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

export function IntRangeEditor(props: {
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

export function FloatRangeEditor(props: {
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

export function ProbabilityField(props: {
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

export function BooleanCard(props: {
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

export function RequestConfigEditor(props: {
  title: string;
  description: string;
  value: RequestFormValue;
  onChange: (value: RequestFormValue) => void;
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

export function asNumber(value: string | number | undefined, fallback: number) {
  const next = Number(value);
  return Number.isFinite(next) ? next : fallback;
}
