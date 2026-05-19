import { useState } from 'react'
import OrderPage from './pages/OrderPage'
import KafkaMonitor from './pages/KafkaMonitor'
import ConsumerGroups from './pages/ConsumerGroups'

const TABS = [
  { key: 'orders', label: 'Create Order', icon: '🛒' },
  { key: 'consumers', label: 'Consumer Groups', icon: '👥' },
  { key: 'monitor', label: 'Kafka Monitor', icon: '📊' },
] as const

type TabKey = typeof TABS[number]['key']

function App() {
  const [currentPage, setCurrentPage] = useState<TabKey>('orders')

  return (
    <div className="min-h-screen bg-gray-100">
      <nav className="bg-white shadow-md">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center gap-2">
          <span className="text-xl font-bold text-gray-800 mr-4">
            ⚡ Kafka Commerce
          </span>
          {TABS.map(tab => (
            <button
              key={tab.key}
              onClick={() => setCurrentPage(tab.key)}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                currentPage === tab.key
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
              }`}
            >
              <span className="mr-1.5">{tab.icon}</span>
              {tab.label}
            </button>
          ))}
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-4 py-8">
        {currentPage === 'orders' && <OrderPage />}
        {currentPage === 'consumers' && <ConsumerGroups />}
        {currentPage === 'monitor' && <KafkaMonitor />}
      </main>
    </div>
  )
}

export default App
