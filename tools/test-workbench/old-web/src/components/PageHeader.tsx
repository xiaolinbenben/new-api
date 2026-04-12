import { Typography } from '@douyinfe/semi-ui';

export function PageHeader(props: { title: string; description: string }) {
  return (
    <div style={{ marginBottom: '8px' }}>
      <Typography.Title heading={3}>{props.title}</Typography.Title>
      <Typography.Text type='tertiary'>{props.description}</Typography.Text>
    </div>
  );
}
