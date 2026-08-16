import { Outlet } from 'react-router-dom'
import { Header } from './components/Header'

export function App() {
  return (
    <div className="min-h-screen bg-gray-50 text-gray-900 antialiased">
      <Header />
      <main className="mx-auto max-w-5xl space-y-6 px-4 py-8">
        <Outlet />
      </main>
    </div>
  )
}
