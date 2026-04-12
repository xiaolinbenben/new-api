import { Suspense, lazy, type ReactNode } from 'react';
import { Navigate, useRoutes, type RouteObject } from 'react-router-dom';
import { AppShell } from './AppShell';
import { navigationItems } from './navigation';

const DashboardPage = lazy(async () => {
  const module = await import('../pages/DashboardPage');
  return { default: module.DashboardPage };
});

const ProjectsPage = lazy(async () => {
  const module = await import('../pages/ProjectsPage');
  return { default: module.ProjectsPage };
});

const MockPage = lazy(async () => {
  const module = await import('../pages/MockPage');
  return { default: module.MockPage };
});

const LoadPage = lazy(async () => {
  const module = await import('../pages/LoadPage');
  return { default: module.LoadPage };
});

const RunsPage = lazy(async () => {
  const module = await import('../pages/RunsPage');
  return { default: module.RunsPage };
});

const NotFoundPage = lazy(async () => {
  const module = await import('../pages/NotFoundPage');
  return { default: module.NotFoundPage };
});

function withSuspense(element: ReactNode) {
  return <Suspense fallback={<div className='route-loading'>页面加载中...</div>}>{element}</Suspense>;
}

const routeObjects: RouteObject[] = [
  {
    path: '/',
    element: <AppShell />,
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

export function AppRoutes() {
  return useRoutes(routeObjects);
}
