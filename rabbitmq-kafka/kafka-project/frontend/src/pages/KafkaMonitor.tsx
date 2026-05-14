import { useState, useEffect } from 'react'

export default function KafkaMonitor() {
  const [kafkaInfo, setKafkaInfo] = useState({
    topics: ['orders'],
    consumers: [
      { name: 'payment-service', group: 'payment-service' },
      { name: 'inventory-service', group: 'inventory-service' },
      { name: 'analytics-service', group: 'analytics-service' },
      { name: 'notification-service', group: 'notification-service' },
    ],
    brokers: ['localhost:9092'],
  })

  useEffect(() => {
    // In Phase 3, we'll add real offset tracking here
    console.log('Kafka Monitor loaded')
  }, [])

  return (
    <div className="space-y-8">
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-2xl font-bold mb-4">Kafka Cluster Info</h2>
        
        <div className="grid grid-cols-3 gap-4 mb-6">
          <div className="bg-blue-50 p-4 rounded">
            <p className="text-sm text-gray-600">Brokers</p>
            <p className="text-2xl font-bold">{kafkaInfo.brokers.length}</p>
          </div>
          <div className="bg-green-50 p-4 rounded">
            <p className="text-sm text-gray-600">Topics</p>
            <p className="text-2xl font-bold">{kafkaInfo.topics.length}</p>
          </div>
          <div className="bg-purple-50 p-4 rounded">
            <p className="text-sm text-gray-600">Consumer Groups</p>
            <p className="text-2xl font-bold">{kafkaInfo.consumers.length}</p>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-2xl font-bold mb-4">Topics</h2>
        <div className="space-y-2">
          {kafkaInfo.topics.map((topic) => (
            <div key={topic} className="border rounded p-3 bg-gray-50">
              <p className="font-medium">{topic}</p>
              <p className="text-sm text-gray-600">Partitions: 1 | Replication Factor: 1</p>
            </div>
          ))}
        </div>
      </div>

      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-2xl font-bold mb-4">Consumer Groups</h2>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b">
                <th className="text-left py-2 px-4">Consumer Group</th>
                <th className="text-left py-2 px-4">State</th>
                <th className="text-left py-2 px-4">Members</th>
              </tr>
            </thead>
            <tbody>
              {kafkaInfo.consumers.map((consumer) => (
                <tr key={consumer.group} className="border-b hover:bg-gray-50">
                  <td className="py-2 px-4 font-mono">{consumer.group}</td>
                  <td className="py-2 px-4">
                    <span className="bg-green-100 text-green-800 px-2 py-1 rounded text-sm">
                      Stable
                    </span>
                  </td>
                  <td className="py-2 px-4">1</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="bg-blue-50 border border-blue-200 rounded p-4">
        <p className="text-sm text-gray-700">
          <strong>Note:</strong> In Phase 3, this page will show real offset tracking and consumer lag. 
          Open <a href="http://localhost:8080" target="_blank" rel="noopener noreferrer" className="text-blue-600 underline">Kafka UI</a> to see live cluster status.
        </p>
      </div>
    </div>
  )
}
