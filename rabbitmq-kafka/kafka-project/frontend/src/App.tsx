import { useState } from 'react'
import OrderPage from './pages/OrderPage'
import KafkaMonitor from './pages/KafkaMonitor'
import './App.css'

function App() {
  const [currentPage, setCurrentPage] = useState('orders')

  return (
    <div className="min-h-screen bg-gray-100">
      <nav className="bg-white shadow-md">
        <div className="max-w-7xl mx-auto px-4 py-4 flex gap-4">
          <button
            onClick={() => setCurrentPage('orders')}
            className={`px-4 py-2 rounded ${
              currentPage === 'orders'
                ? 'bg-blue-600 text-white'
                : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
            }`}
          >
            Create Order
          </button>
          <button
            onClick={() => setCurrentPage('monitor')}
            className={`px-4 py-2 rounded ${
              currentPage === 'monitor'
                ? 'bg-blue-600 text-white'
                : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
            }`}
          >
            Kafka Monitor
          </button>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-4 py-8">
        {currentPage === 'orders' && <OrderPage />}
        {currentPage === 'monitor' && <KafkaMonitor />}
      </main>
    </div>
  )
}

export default App
