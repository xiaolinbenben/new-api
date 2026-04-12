import { Outlet } from 'react-router-dom'

export default function Layout() {
  return (
    <div style={{ display: 'flex', height: '100vh' }}>
      <aside style={{ width: 200, background: '#f5f5f5', padding: 20 }}>
        <h3>Menu</h3>
      </aside>
      <main style={{ flex: 1, padding: 20 }}>
        <Outlet />
      </main>
    </div>
  )
}
