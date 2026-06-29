import { useState, useEffect } from 'react'
import axios from 'axios'

interface ConsumerGroup {
  group_id: string
  members: { instance_id: string; status: string; partitions: number[]; messages_read: number }[]
  state: string
}

export default function KafkaMonitor() {
  const [groups, setGroups] = useState<ConsumerGroup[]>([])

  const kafkaInfo = {
    topics: ['orders'],
    brokers: ['localhost:9092'],
    partitions: 3,
  }

  useEffect(() => {
    const fetchGroups = async () => {
      try {
        const res = await axios.get('/api/consumer-groups')
        setGroups(res.data || [])
      } catch (err) {
        console.error('Error fetching consumer groups:', err)
      }
    }

    fetchGroups()
    const interval = setInterval(fetchGroups, 3000)
    return () => clearInterval(interval)
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
            <p className="text-2xl font-bold">{groups.length}</p>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-2xl font-bold mb-4">Topics</h2>
        <div className="space-y-2">
          {kafkaInfo.topics.map((topic) => (
            <div key={topic} className="border rounded p-3 bg-gray-50">
              <p className="font-medium">{topic}</p>
              <p className="text-sm text-gray-600">Partitions: {kafkaInfo.partitions} | Replication Factor: 1</p>
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
                <th className="text-right py-2 px-4">Messages</th>
              </tr>
            </thead>
            <tbody>
              {groups.length === 0 ? (
                <tr>
                  <td colSpan={4} className="text-center py-4 text-gray-400">
                    Waiting for consumer groups...
                  </td>
                </tr>
              ) : (
                groups.map((group) => (
                  <tr key={group.group_id} className="border-b hover:bg-gray-50">
                    <td className="py-2 px-4 font-mono">{group.group_id}</td>
                    <td className="py-2 px-4">
                      <span className={`px-2 py-1 rounded text-sm ${
                        group.state === 'Stable'
                          ? 'bg-green-100 text-green-800'
                          : group.state === 'Rebalancing'
                          ? 'bg-amber-100 text-amber-800'
                          : 'bg-gray-100 text-gray-600'
                      }`}>
                        {group.state}
                      </span>
                    </td>
                    <td className="py-2 px-4">{group.members.length}</td>
                    <td className="py-2 px-4 text-right font-mono">
                      {group.members.reduce((s, m) => s + m.messages_read, 0).toLocaleString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className="bg-blue-50 border border-blue-200 rounded p-4">
        <p className="text-sm text-gray-700">
          <strong>Phase 2 Active:</strong> Consumer groups now have multiple instances (3 per service by default). 
          Use the <strong>Consumer Groups</strong> tab for detailed partition assignments and rebalance events. 
          Open <a href="http://localhost:8080" target="_blank" rel="noopener noreferrer" className="text-blue-600 underline">Kafka UI</a> to see the live cluster status.
        </p>
      </div>
    </div>
  )
}
