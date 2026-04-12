import { Suspense, lazy, type ReactNode } from 'react';
import { Navigate, type RouteObject, useRoutes } from 'react-router-dom';
import { WorkbenchLayout } from '../layouts';

const DashboardPage = lazy(async () => {
  const module = await import('../pages/dashboard');
  return { default: module.DashboardPage };
});

const ProjectsPage = lazy(async () => {
  const module = await import('../pages/projects');
  return { default: module.ProjectsPage };
});

const MockPage = lazy(async () => {
  const module = await import('../pages/mock');
  return { default: module.MockPage };
});

const LoadPage = lazy(async () => {
  const module = await import('../pages/load');
  return { default: module.LoadPage };
});

const RunsPage = lazy(async () => {
  const module = await import('../pages/runs');
  return { default: module.RunsPage };
});

const NotFoundPage = lazy(async () => {
  const module = await import('../pages/not-found');
  return { default: module.NotFoundPage };
});

function withSuspense(element: ReactNode) {
  return <Suspense fallback={<div style={{ padding: '40px 0', color: 'rgba(26, 36, 48, 0.72)', fontSize: '14px' }}>页面加载中...</div>}>{element}</Suspense>;
}

const routeObjects: RouteObject[] = [
  {
    path: '/',
    element: <WorkbenchLayout />,
    children: [
      { index: true, element: <Navigate to='dashboard' replace /> },
      { path: 'dashboard', element: withSuspense(<DashboardPage />) },
      { path: 'projects', element: withSuspense(<ProjectsPage />) },
      { path: 'mock', element: withSuspense(<MockPage />) },
      { path: 'load', element: withSuspense(<LoadPage />) },
      { path: 'runs', element: withSuspense(<RunsPage />) },
      { path: '*', element: withSuspense(<NotFoundPage />) }
    ]
  }
];

export function AppRouter() {
  return useRoutes(routeObjects);
}
