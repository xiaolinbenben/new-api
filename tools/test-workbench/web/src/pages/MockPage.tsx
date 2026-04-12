import { Button, Card, Select, Space, Table, Tag, TextArea, Typography } from '@douyinfe/semi-ui';
import { PageHeader } from '../components/PageHeader';
import { MockProfileEditor } from '../config-editors';
import { useWorkbench } from '../features/workbench/context';
import { formatStatus, safePretty, statusColor } from '../features/workbench/helpers';

export function MockPage() {
  const { queries, drafts, selection, mutations, actions } = useWorkbench();

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <PageHeader
        title='Mock 配置'
        description='Mock 页只处理模拟器行为和运行态观察。配置编辑、监听器控制和事件观察全部收拢在一个页面里。'
      />

      <Card className='panel wide'>
        <div className='page-toolbar'>
          <Select
            value={selection.selectedMockProfileId}
            style={{ width: 320 }}
            optionList={(queries.mockProfilesQuery.data ?? []).map((item) => ({ value: item.id, label: item.name }))}
            onChange={(value) => selection.setSelectedMockProfileId(String(value))}
          />
          <Button onClick={actions.duplicateMockProfile}>复制并新建 Mock 配置</Button>
          <Button theme='solid' onClick={() => mutations.saveMockProfile.mutate()} disabled={!drafts.mockProfileDraft}>
            保存 Mock 配置
          </Button>
          <Button theme='solid' type='primary' onClick={() => mutations.startMockListener.mutate()} disabled={!selection.selectedEnvironmentId}>
            启动当前环境监听器
          </Button>
          <Button type='danger' onClick={() => mutations.stopMockListener.mutate()} disabled={!selection.selectedEnvironmentId}>
            停止当前环境监听器
          </Button>
        </div>
      </Card>

      <Card className='panel wide'>
        {drafts.mockProfileDraft ? (
          <MockProfileEditor value={drafts.mockProfileDraft} onChange={drafts.setMockProfileDraft} onSave={() => mutations.saveMockProfile.mutate()} />
        ) : (
          <Typography.Text type='tertiary'>请选择一个 Mock 配置。</Typography.Text>
        )}
      </Card>

      <div className='dashboard-grid'>
        <Card className='panel'>
          <Typography.Title heading={5}>活动监听器</Typography.Title>
          <Table
            pagination={false}
            dataSource={queries.mockListenersQuery.data ?? []}
            columns={[
              { title: '环境', dataIndex: 'name' },
              { title: '状态', dataIndex: 'status', render: (value) => <Tag color={statusColor(value)}>{formatStatus(value)}</Tag> },
              { title: '本地地址', dataIndex: 'local_base_url' },
              { title: 'QPS', render: (_, record) => Number(record.summary.current_qps ?? 0).toFixed(2) }
            ]}
          />
        </Card>
        <Card className='panel'>
          <Typography.Title heading={5}>路由统计</Typography.Title>
          <Table
            pagination={false}
            dataSource={queries.routesQuery.data ?? []}
            columns={[
              { title: '路由', dataIndex: 'route' },
              { title: '请求数', dataIndex: 'requests' },
              { title: '错误数', dataIndex: 'errors' },
              { title: 'P95', dataIndex: 'p95_ms', render: (value) => `${Number(value ?? 0).toFixed(1)} ms` }
            ]}
          />
        </Card>
      </div>

      <Card className='panel wide'>
        <Typography.Title heading={5}>最近事件</Typography.Title>
        <Typography.Text type='tertiary'>保留原始事件结构，方便你在排查时直接复制请求、错误或视频任务详情。</Typography.Text>
        <TextArea rows={12} value={safePretty(queries.eventsQuery.data ?? { requests: [], errors: [], videos: [] })} readOnly />
      </Card>
    </Space>
  );
}
