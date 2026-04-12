import { Link } from 'react-router-dom';
import { PageHeader } from '../../components/PageHeader';

export function NotFoundPage() {
  return (
    <div>
      <PageHeader title='页面不存在' description='当前路由没有匹配到可用页面，你可以返回总览重新进入工作台。' />
      <Link to='/dashboard'>打开总览</Link>
    </div>
  );
}
