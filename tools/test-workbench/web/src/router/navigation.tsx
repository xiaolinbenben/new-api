import { IconActivity, IconAppCenter, IconApps, IconCloud, IconServer } from '@douyinfe/semi-icons';

export const navigationItems = [
  { itemKey: '/dashboard', text: '总览', icon: <IconActivity /> },
  { itemKey: '/projects', text: '项目与环境', icon: <IconAppCenter /> },
  { itemKey: '/mock', text: 'Mock 配置', icon: <IconCloud /> },
  { itemKey: '/load', text: '压测配置', icon: <IconServer /> },
  { itemKey: '/runs', text: '运行记录', icon: <IconApps /> }
];
