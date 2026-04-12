import { Button, Card, Select, Space, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { PageHeader } from '../../components/PageHeader';
import { useWorkbench } from '../../features/workbench/context';
import { formatStatus, statusColor } from '../../features/workbench/helpers';
import { RunProfileForm, ScenarioForm } from './form';

export function LoadPage() {
  const { queries, drafts, selection, mutations, actions } = useWorkbench();

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <PageHeader
        title='压测配置'
        description='压测页拆成“运行参数模板”和“场景模板”两部分，后续再扩展压测能力时，不需要再改动整体布局。'
      />

      <Card className='panel wide'>
        <div className='page-toolbar'>
          <Select
            value={selection.selectedRunProfileId}
            style={{ width: 320 }}
            optionList={(queries.runProfilesQuery.data ?? []).map((item) => ({ value: item.id, label: item.name }))}
            onChange={(value) => selection.setSelectedRunProfileId(String(value))}
          />
          <Button onClick={actions.duplicateRunProfile}>复制并新建压测配置</Button>
          <Button theme='solid' onClick={() => mutations.saveRunProfile.mutate()} disabled={!drafts.runProfileDraft}>
            保存压测配置
          </Button>
          <Button theme='solid' type='primary' onClick={() => mutations.startRun.mutate()} disabled={!selection.selectedEnvironmentId}>
            启动压测
          </Button>
          <Button type='danger' disabled={!selection.selectedRunId} onClick={() => mutations.stopRun.mutate()}>
            停止当前压测
          </Button>
        </div>
      </Card>

      <Card className='panel wide'>
        {drafts.runProfileDraft ? (
          <RunProfileForm value={drafts.runProfileDraft} onChange={drafts.setRunProfileDraft} onSave={() => mutations.saveRunProfile.mutate()} />
        ) : (
          <Typography.Text type='tertiary'>请选择一个压测配置。</Typography.Text>
        )}
      </Card>

      <Card className='panel wide'>
        <div className='section-headline'>
          <div>
            <Typography.Title heading={5}>场景选择</Typography.Title>
            <Typography.Text type='tertiary'>场景负责定义请求形态。你可以在这里管理单次请求、流式输出和任务轮询三种模型。</Typography.Text>
          </div>
          <Space>
            <Select
              value={selection.selectedScenarioId}
              style={{ width: 320 }}
              optionList={(queries.scenariosQuery.data ?? []).map((item) => ({ value: item.id, label: item.name }))}
              onChange={(value) => selection.setSelectedScenarioId(String(value))}
            />
            <Button onClick={actions.duplicateScenario}>复制并新建场景</Button>
            <Button theme='solid' onClick={() => mutations.saveScenario.mutate()} disabled={!drafts.scenarioDraft}>
              保存场景
            </Button>
          </Space>
        </div>
      </Card>

      <Card className='panel wide'>
        {drafts.scenarioDraft ? (
          <ScenarioForm value={drafts.scenarioDraft} onChange={drafts.setScenarioDraft} onSave={() => mutations.saveScenario.mutate()} />
        ) : (
          <Typography.Text type='tertiary'>请选择一个场景。</Typography.Text>
        )}
      </Card>

      <Card className='panel wide'>
        <Typography.Title heading={5}>活动压测</Typography.Title>
        <Table
          pagination={false}
          dataSource={queries.loadRunsQuery.data ?? []}
          columns={[
            {
              title: '运行 ID',
              dataIndex: 'run_id',
              render: (value) => (
                <Button theme='borderless' onClick={() => selection.setSelectedRunId(value)}>
                  {value}
                </Button>
              )
            },
            { title: '状态', dataIndex: 'status', render: (value) => <Tag color={statusColor(value)}>{formatStatus(value)}</Tag> },
            { title: '目标地址', dataIndex: 'target_base_url' },
            { title: 'TPS', render: (_, record) => Number(record.summary.current_tps ?? 0).toFixed(2) },
            { title: '错误数', render: (_, record) => record.summary.errors }
          ]}
        />
      </Card>
    </Space>
  );
}
