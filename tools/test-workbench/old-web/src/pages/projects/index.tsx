import { Button, Card, Input, Space, Table, Typography } from '@douyinfe/semi-ui';
import { PageHeader } from '../../components/PageHeader';
import { useWorkbench } from '../../features/workbench/context';
import { formatTargetType } from '../../features/workbench/helpers';
import { EnvironmentForm } from './form';

export function ProjectsPage() {
  const { forms, queries, drafts, selection, mutations, actions } = useWorkbench();

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <PageHeader
        title='项目与环境'
        description='项目页只负责项目和环境。环境的详细表单、默认目标、鉴权和监听器绑定都集中在这里管理。'
      />

      <Card style={{ width: '100%', borderRadius: '22px', border: '1px solid rgba(26, 36, 48, 0.08)', background: 'rgba(255, 252, 245, 0.88)', backdropFilter: 'blur(10px)', boxShadow: '0 24px 60px rgba(32, 43, 52, 0.08)' }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
          <Input value={forms.projectName} onChange={forms.setProjectName} placeholder='新项目名称' />
          <Input value={forms.projectDescription} onChange={forms.setProjectDescription} placeholder='项目描述' />
          <Button theme='solid' onClick={() => mutations.createProject.mutate()}>
            创建项目
          </Button>
        </div>
      </Card>

      <Card style={{ width: '100%', borderRadius: '22px', border: '1px solid rgba(26, 36, 48, 0.08)', background: 'rgba(255, 252, 245, 0.88)', backdropFilter: 'blur(10px)', boxShadow: '0 24px 60px rgba(32, 43, 52, 0.08)' }}>
        <div style={{ width: '100%', display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '12px' }}>
          <div>
            <Typography.Title heading={5}>环境列表</Typography.Title>
            <Typography.Text type='tertiary'>
              先从列表里切换环境，再在下方的结构化表单里编辑。新环境草稿在点击“保存配置”前不会写入数据库。
            </Typography.Text>
          </div>
          <Button onClick={actions.createEnvironmentDraft}>新建环境</Button>
        </div>
        <Table
          pagination={false}
          dataSource={queries.environmentsQuery.data ?? []}
          rowKey='id'
          columns={[
            {
              title: '名称',
              dataIndex: 'name',
              render: (value, record) => (
                <Button theme='borderless' onClick={() => selection.setSelectedEnvironmentId(record.id)}>
                  {value}
                </Button>
              )
            },
            { title: '目标类型', dataIndex: 'target_type', render: (value) => formatTargetType(value) },
            { title: 'Mock 端口', dataIndex: 'mock_port' },
            { title: '自动启动', dataIndex: 'auto_start', render: (value) => (value ? '是' : '否') }
          ]}
        />
      </Card>

      <Card style={{ width: '100%', borderRadius: '22px', border: '1px solid rgba(26, 36, 48, 0.08)', background: 'rgba(255, 252, 245, 0.88)', backdropFilter: 'blur(10px)', boxShadow: '0 24px 60px rgba(32, 43, 52, 0.08)' }}>
        {drafts.environmentDraft ? (
          <EnvironmentForm
            value={drafts.environmentDraft}
            mockProfiles={queries.mockProfilesQuery.data ?? []}
            runProfiles={queries.runProfilesQuery.data ?? []}
            onChange={drafts.setEnvironmentDraft}
            onSave={() => mutations.saveEnvironment.mutate()}
            onDelete={drafts.environmentDraft.id ? actions.deleteEnvironment : undefined}
          />
        ) : (
          <Typography.Text type='tertiary'>请选择一个环境，或者先新建一个环境。</Typography.Text>
        )}
      </Card>
    </Space>
  );
}
