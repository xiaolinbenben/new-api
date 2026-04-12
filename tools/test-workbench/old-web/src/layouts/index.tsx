import { Layout, Nav, Select, Space, Tag } from '@douyinfe/semi-ui';
import { IconTreeTriangleDown } from '@douyinfe/semi-icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useWorkbench } from '../features/workbench/context';
import { navigationItems } from '../router/navigation';

const { Header, Sider, Content } = Layout;

export function WorkbenchLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { queries, selection, selected } = useWorkbench();

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider style={{ background: 'linear-gradient(180deg, #143642 0%, #1d3f49 100%)', color: 'white', borderRight: '1px solid rgba(255, 255, 255, 0.08)' }}>
        <div style={{ padding: '28px 20px 12px' }}>
          <div style={{ fontSize: '28px', fontWeight: 700 }}>测试工作台</div>
        </div>
        <Nav
          selectedKeys={[location.pathname]}
          items={navigationItems}
          onSelect={(data) => navigate(String(data.itemKey))}
          footer={
            <div style={{ margin: '16px', padding: '12px 14px', borderRadius: '14px', background: 'rgba(255, 255, 255, 0.08)', display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <span>当前项目</span>
              <strong>{selected.selectedProject?.name ?? '加载中...'}</strong>
            </div>
          }
        />
      </Sider>
      <Layout>
        <Header style={{ background: 'transparent', padding: '18px 24px 0' }}>
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
        <Content style={{ padding: '20px 24px 32px' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
