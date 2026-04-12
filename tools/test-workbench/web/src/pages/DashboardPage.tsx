import { useMemo } from 'react';
import { Button, Card, Descriptions, Space, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { useNavigate } from 'react-router-dom';
import { PageHeader } from '../components/PageHeader';
import { StatCard } from '../components/StatCard';
import { useWorkbench } from '../features/workbench/context';
import { formatStatus, statusColor } from '../features/workbench/helpers';

export function DashboardPage() {
  const navigate = useNavigate();
  const { queries, selected, selection } = useWorkbench();

  const statusChartSpec = useMemo(() => {
    const source = queries.runDetailQuery.data?.summary.status_codes ?? selected.selectedListener?.summary.status_codes ?? {};
    const values = Object.entries(source).map(([code, count]) => ({ code, count }));
    return {
      type: 'bar',
      data: [{ id: 'status', values }],
      xField: 'code',
      yField: 'count',
      seriesField: 'code'
    };
  }, [queries.runDetailQuery.data, selected.selectedListener]);

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <PageHeader
        title='总览'
        description='从这里快速查看当前项目的环境状态、Mock 监听器健康度、最近运行结果，以及压测总体表现。'
      />

      <div className='card-grid'>
        <StatCard title='项目数' value={queries.projectsQuery.data?.length ?? 0} subtitle='当前工作台中的配置空间' />
        <StatCard title='Mock 请求数' value={selected.selectedListener?.summary.total_requests ?? 0} subtitle='当前环境监听器累计请求' />
        <StatCard title='当前 QPS' value={(selected.selectedListener?.summary.current_qps ?? 0).toFixed(2)} subtitle='内置模拟器实时吞吐' />
        <StatCard title='实时 TPS' value={(queries.loadRunsQuery.data?.[0]?.summary.current_tps ?? 0).toFixed(2)} subtitle='压测实时输出速率' />
      </div>

      <div className='dashboard-grid'>
        <Card className='panel'>
          <Descriptions
            title='运行态概览'
            data={[
              { key: '当前环境', value: selected.selectedEnvironment?.name ?? '无' },
              { key: '监听器地址', value: selected.selectedListener?.listen_address ?? '未运行' },
              {
                key: '压测目标',
                value:
                  queries.loadRunsQuery.data?.[0]?.target_base_url ??
                  selected.selectedEnvironment?.external_base_url ??
                  selected.selectedListener?.local_base_url ??
                  '无'
              },
              { key: '活动压测', value: queries.loadRunsQuery.data?.[0]?.run_id ?? '暂无活动压测' }
            ]}
          />
        </Card>
        <Card className='panel'>
          <Card className='inner-panel'>
            <VChart spec={statusChartSpec as never} style={{ height: 280 }} />
          </Card>
        </Card>
      </div>

      <Card className='panel wide'>
        <Typography.Title heading={5}>最近运行</Typography.Title>
        <Table
          pagination={false}
          dataSource={queries.runsQuery.data ?? []}
          columns={[
            {
              title: '运行 ID',
              dataIndex: 'id',
              render: (value) => (
                <Button
                  theme='borderless'
                  onClick={() => {
                    selection.setSelectedRunId(value);
                    navigate('/runs');
                  }}
                >
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
    </Space>
  );
}
