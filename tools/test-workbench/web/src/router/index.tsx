import { createBrowserRouter } from 'react-router-dom'
import Layout from '@/layout'
import Home from '@/pages/home/index'
import About from '@/pages/about/index'

const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      {
        index: true,
        element: <Home />,
      },
      {
        path: 'about',
        element: <About />,
      },
    ],
  },
])

export default router
