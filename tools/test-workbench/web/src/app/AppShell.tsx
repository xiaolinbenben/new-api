import { Layout, Nav, Select, Space, Tag } from '@douyinfe/semi-ui';
import { IconTreeTriangleDown } from '@douyinfe/semi-icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useWorkbench } from '../features/workbench/context';
import { navigationItems } from './navigation';

const { Header, Sider, Content } = Layout;

export function AppShell() {
  const location = useLocation();
  const navigate = useNavigate();
  const { queries, selection, selected } = useWorkbench();

  return (
    <Layout className='shell'>
      <Sider className='shell-sider'>
        <div className='brand'>
          <div className='brand-kicker'>new-api</div>
          <div className='brand-title'>测试工作台</div>
          <div className='brand-copy'>结构化配置、Mock 模拟、压测与回放</div>
        </div>
        <Nav
          selectedKeys={[location.pathname]}
          items={navigationItems}
          onSelect={(data) => navigate(String(data.itemKey))}
          footer={
            <div className='project-pill'>
              <span>当前项目</span>
              <strong>{selected.selectedProject?.name ?? '加载中...'}</strong>
            </div>
          }
        />
      </Sider>
      <Layout>
        <Header className='shell-header'>
          <Space>
            <Select
              value={selection.selectedProjectId}
              style={{ width: 280 }}
              optionList={(queries.projectsQuery.data ?? []).map((item) => ({ value: item.id, label: item.name }))}
              onChange={(value) => selection.setSelectedProjectId(String(value))}
              suffix={<IconTreeTriangleDown />}
            />
            <Tag color='cyan'>管理 API /api/v1</Tag>
            <Tag color='teal'>{queries.loadRunsQuery.data?.length ?? 0} 个活动压测</Tag>
            <Tag color='orange'>{queries.mockListenersQuery.data?.length ?? 0} 个活动 Mock 监听器</Tag>
          </Space>
        </Header>
        <Content className='shell-content'>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
