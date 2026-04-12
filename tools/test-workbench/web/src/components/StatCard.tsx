import { Card, Typography } from '@douyinfe/semi-ui';

export function StatCard(props: { title: string; value: string | number; subtitle: string }) {
  return (
    <Card className='stat-card'>
      <Typography.Text type='tertiary'>{props.title}</Typography.Text>
      <Typography.Title heading={2}>{props.value}</Typography.Title>
      <Typography.Text>{props.subtitle}</Typography.Text>
    </Card>
  );
}
