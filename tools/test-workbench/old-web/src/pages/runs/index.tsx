import { useMemo } from 'react';
import { Button, Card, Space, Table, Tag, TextArea, Typography } from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { PageHeader } from '../../components/PageHeader';
import { StatCard } from '../../components/StatCard';
import { useWorkbench } from '../../features/workbench/context';
import { formatScenarioMode, formatStatus, safePretty, statusColor } from '../../features/workbench/helpers';

export function RunsPage() {
  const { queries, selection } = useWorkbench();

  const scenarioChartSpec = useMemo(() => {
    const values =
      queries.runDetailQuery.data?.scenarios.flatMap((item) => [
        { name: item.name, type: '请求数', value: item.requests },
        { name: item.name, type: '错误数', value: item.errors }
      ]) ?? [];

    return {
      type: 'bar',
      data: [{ id: 'scenario', values }],
      xField: 'name',
      yField: 'value',
      seriesField: 'type'
    };
  }, [queries.runDetailQuery.data]);

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <PageHeader
        title='运行记录'
        description='运行记录页独立负责历史查询、统计图和样本回放。后续如果要加筛选器、分页或导出，也只需要改这里。'
      />

      <div style={{ width: '100%', display: 'grid', gridTemplateColumns: 'minmax(320px, 0.9fr) minmax(420px, 1.1fr)', gap: '16px' }}>
        <Card style={{ width: '100%', borderRadius: '22px', border: '1px solid rgba(26, 36, 48, 0.08)', background: 'rgba(255, 252, 245, 0.88)', backdropFilter: 'blur(10px)', boxShadow: '0 24px 60px rgba(32, 43, 52, 0.08)' }}>
          <Typography.Title heading={5}>运行历史</Typography.Title>
          <Table
            pagination={false}
            dataSource={queries.runsQuery.data ?? []}
            rowKey='id'
            columns={[
              {
                title: '运行 ID',
                dataIndex: 'id',
                render: (value) => (
                  <Button theme='borderless' onClick={() => selection.setSelectedRunId(value)}>
                    {value}
                  </Button>
                )
              },
              { title: '状态', dataIndex: 'status', render: (value) => <Tag color={statusColor(value)}>{formatStatus(value)}</Tag> },
              { title: '请求数', dataIndex: 'total_requests' },
              { title: '错误数', dataIndex: 'errors' },
              { title: 'P95', dataIndex: 'p95_ms', render: (value) => `${Number(value ?? 0).toFixed(1)} ms` }
            ]}
          />
        </Card>

        <Card style={{ width: '100%', borderRadius: '22px', border: '1px solid rgba(26, 36, 48, 0.08)', background: 'rgba(255, 252, 245, 0.88)', backdropFilter: 'blur(10px)', boxShadow: '0 24px 60px rgba(32, 43, 52, 0.08)' }}>
          <Typography.Title heading={5}>运行摘要</Typography.Title>
          {queries.runDetailQuery.data ? (
            <>
              <div style={{ width: '100%', display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: '16px', marginBottom: '16px' }}>
                <StatCard title='请求数' value={queries.runDetailQuery.data.summary.total_requests} subtitle='累计请求' />
                <StatCard title='错误数' value={queries.runDetailQuery.data.summary.errors} subtitle='失败请求' />
                <StatCard title='P95' value={`${Number(queries.runDetailQuery.data.summary.p95_ms ?? 0).toFixed(1)} ms`} subtitle='延迟' />
                <StatCard title='TPS' value={`${Number(queries.runDetailQuery.data.summary.current_tps ?? 0).toFixed(2)}`} subtitle='实时输出' />
              </div>
              <Card style={{ width: '100%', borderRadius: '22px', border: '1px solid rgba(26, 36, 48, 0.08)', background: 'rgba(255, 252, 245, 0.88)', backdropFilter: 'blur(10px)', boxShadow: '0 24px 60px rgba(32, 43, 52, 0.08)' }}>
                <Typography.Title heading={6}>场景拆分</Typography.Title>
                <VChart spec={scenarioChartSpec as never} style={{ height: 260 }} />
                <Table
                  pagination={false}
                  dataSource={queries.runDetailQuery.data.scenarios}
                  columns={[
                    { title: '场景', dataIndex: 'name' },
                    { title: '模式', dataIndex: 'mode', render: (value) => formatScenarioMode(value) },
                    { title: '请求数', dataIndex: 'requests' },
                    { title: '错误数', dataIndex: 'errors' },
                    { title: 'P95', dataIndex: 'p95_ms', render: (value) => `${Number(value ?? 0).toFixed(1)} ms` }
                  ]}
                />
              </Card>
            </>
          ) : (
            <Typography.Text type='tertiary'>请选择一个运行记录。</Typography.Text>
          )}
        </Card>
      </div>

      <Card style={{ width: '100%', borderRadius: '22px', border: '1px solid rgba(26, 36, 48, 0.08)', background: 'rgba(255, 252, 245, 0.88)', backdropFilter: 'blur(10px)', boxShadow: '0 24px 60px rgba(32, 43, 52, 0.08)' }}>
        <Typography.Title heading={5}>样本</Typography.Title>
        <Typography.Text type='tertiary'>样本继续保留原始结构，方便精确复制请求、响应和错误信息进行复现。</Typography.Text>
        <TextArea rows={14} readOnly value={safePretty(queries.runDetailQuery.data?.samples ?? { requests: [], errors: [] })} />
      </Card>
    </Space>
  );
}
