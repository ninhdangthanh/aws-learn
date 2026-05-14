import { useState, useEffect, useCallback } from 'react'
import axios from 'axios'

interface ConsumerInstance {
  group_id: string
  instance_id: string
  status: string
  partitions: number[]
  messages_read: number
  last_message: string
  started_at: string
}

interface ConsumerGroup {
  group_id: string
  members: ConsumerInstance[]
  state: string
}

interface RebalanceEvent {
  group_id: string
  instance_id: string
  event_type: string
  partitions: number[]
  timestamp: string
}

const STATUS_COLORS: Record<string, { bg: string; text: string; dot: string }> = {
  active: { bg: 'bg-emerald-50', text: 'text-emerald-700', dot: 'bg-emerald-500' },
  rebalancing: { bg: 'bg-amber-50', text: 'text-amber-700', dot: 'bg-amber-500' },
  stopped: { bg: 'bg-red-50', text: 'text-red-700', dot: 'bg-red-500' },
}

const GROUP_STATE_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  Stable: { bg: 'bg-emerald-100', text: 'text-emerald-800', border: 'border-emerald-300' },
  Rebalancing: { bg: 'bg-amber-100', text: 'text-amber-800', border: 'border-amber-300' },
  Empty: { bg: 'bg-gray-100', text: 'text-gray-600', border: 'border-gray-300' },
}

const EVENT_TYPE_COLORS: Record<string, { bg: string; text: string; icon: string }> = {
  joined: { bg: 'bg-blue-50', text: 'text-blue-700', icon: '→' },
  left: { bg: 'bg-red-50', text: 'text-red-700', icon: '←' },
  assigned: { bg: 'bg-emerald-50', text: 'text-emerald-700', icon: '✓' },
  revoked: { bg: 'bg-amber-50', text: 'text-amber-700', icon: '✗' },
}

const PARTITION_COLORS = [
  'bg-blue-500',
  'bg-emerald-500',
  'bg-violet-500',
  'bg-amber-500',
  'bg-rose-500',
  'bg-cyan-500',
  'bg-orange-500',
  'bg-pink-500',
]

export default function ConsumerGroups() {
  const [groups, setGroups] = useState<ConsumerGroup[]>([])
  const [events, setEvents] = useState<RebalanceEvent[]>([])
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date())

  const fetchData = useCallback(async () => {
    try {
      const [groupsRes, eventsRes] = await Promise.all([
        axios.get('/api/consumer-groups'),
        axios.get('/api/rebalance-events?limit=50'),
      ])
      setGroups(groupsRes.data || [])
      setEvents(eventsRes.data || [])
      setLastUpdated(new Date())
    } catch (error) {
      console.error('Error fetching consumer data:', error)
    }
  }, [])

  useEffect(() => {
    fetchData()
    if (!autoRefresh) return

    const interval = setInterval(fetchData, 2000)
    return () => clearInterval(interval)
  }, [fetchData, autoRefresh])

  // Compute summary stats
  const totalInstances = groups.reduce((sum, g) => sum + g.members.length, 0)
  const totalMessages = groups.reduce(
    (sum, g) => sum + g.members.reduce((s, m) => s + m.messages_read, 0),
    0
  )
  const allPartitions = new Set<number>()
  groups.forEach(g =>
    g.members.forEach(m => m.partitions?.forEach(p => allPartitions.add(p)))
  )

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Consumer Groups</h2>
          <p className="text-sm text-gray-500 mt-1">
            Phase 2 — Horizontal scaling & partition rebalancing
          </p>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-400">
            Updated {lastUpdated.toLocaleTimeString()}
          </span>
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`px-3 py-1.5 text-xs font-medium rounded-full transition-all ${
              autoRefresh
                ? 'bg-emerald-100 text-emerald-700 ring-1 ring-emerald-300'
                : 'bg-gray-100 text-gray-600 ring-1 ring-gray-300'
            }`}
          >
            {autoRefresh ? '● Live' : '○ Paused'}
          </button>
          <button
            onClick={fetchData}
            className="px-3 py-1.5 text-xs font-medium rounded-full bg-blue-100 text-blue-700 ring-1 ring-blue-300 hover:bg-blue-200 transition-all"
          >
            ↻ Refresh
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-4 gap-4">
        <SummaryCard
          label="Consumer Groups"
          value={groups.length}
          color="blue"
        />
        <SummaryCard
          label="Total Instances"
          value={totalInstances}
          color="emerald"
        />
        <SummaryCard
          label="Active Partitions"
          value={allPartitions.size}
          color="violet"
        />
        <SummaryCard
          label="Messages Processed"
          value={totalMessages}
          color="amber"
        />
      </div>

      {/* Partition Distribution Visualization */}
      {groups.length > 0 && (
        <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
          <h3 className="text-lg font-semibold text-gray-900 mb-4">
            Partition Distribution
          </h3>
          <div className="space-y-4">
            {groups.map(group => (
              <PartitionBar key={group.group_id} group={group} />
            ))}
          </div>
          <div className="flex items-center gap-4 mt-4 pt-4 border-t border-gray-100">
            {Array.from(allPartitions)
              .sort()
              .map(p => (
                <div key={p} className="flex items-center gap-1.5">
                  <div
                    className={`w-3 h-3 rounded-sm ${
                      PARTITION_COLORS[p % PARTITION_COLORS.length]
                    }`}
                  />
                  <span className="text-xs text-gray-500">P{p}</span>
                </div>
              ))}
          </div>
        </div>
      )}

      {/* Consumer Groups Detail */}
      <div className="space-y-4">
        {groups.length === 0 ? (
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-12 text-center">
            <div className="text-4xl mb-3">📡</div>
            <p className="text-gray-500 font-medium">
              No consumer groups registered yet
            </p>
            <p className="text-sm text-gray-400 mt-1">
              Start the backend to see consumer instances appear
            </p>
          </div>
        ) : (
          groups.map(group => (
            <ConsumerGroupCard
              key={group.group_id}
              group={group}
              isExpanded={expandedGroup === group.group_id}
              onToggle={() =>
                setExpandedGroup(
                  expandedGroup === group.group_id ? null : group.group_id
                )
              }
            />
          ))
        )}
      </div>

      {/* Rebalance Events */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
        <h3 className="text-lg font-semibold text-gray-900 mb-4">
          Rebalance Events
          {events.length > 0 && (
            <span className="ml-2 text-xs font-normal bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full">
              {events.length}
            </span>
          )}
        </h3>
        {events.length === 0 ? (
          <p className="text-sm text-gray-400 py-4 text-center">
            No rebalance events yet — create orders to trigger consumer activity
          </p>
        ) : (
          <div className="space-y-2 max-h-80 overflow-y-auto">
            {events.map((event, idx) => (
              <RebalanceEventRow key={idx} event={event} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

/* ─── Sub-components ──────────────────────────────────────────────────── */

function SummaryCard({
  label,
  value,
  color,
}: {
  label: string
  value: number
  color: string
}) {
  const bgMap: Record<string, string> = {
    blue: 'bg-blue-50 border-blue-200',
    emerald: 'bg-emerald-50 border-emerald-200',
    violet: 'bg-violet-50 border-violet-200',
    amber: 'bg-amber-50 border-amber-200',
  }
  const textMap: Record<string, string> = {
    blue: 'text-blue-700',
    emerald: 'text-emerald-700',
    violet: 'text-violet-700',
    amber: 'text-amber-700',
  }
  return (
    <div
      className={`rounded-xl border p-4 ${bgMap[color] || bgMap.blue}`}
    >
      <p className="text-xs font-medium text-gray-500 uppercase tracking-wide">
        {label}
      </p>
      <p className={`text-3xl font-bold mt-1 ${textMap[color] || textMap.blue}`}>
        {value.toLocaleString()}
      </p>
    </div>
  )
}

function PartitionBar({ group }: { group: ConsumerGroup }) {
  // Build a map of partition -> instance
  const partitionMap = new Map<number, string>()
  group.members.forEach(m => {
    m.partitions?.forEach(p => {
      partitionMap.set(p, m.instance_id)
    })
  })

  const partitions = Array.from(partitionMap.keys()).sort()
  if (partitions.length === 0) return null

  return (
    <div>
      <div className="flex items-center gap-2 mb-1.5">
        <span className="text-sm font-medium text-gray-700 w-44 truncate">
          {group.group_id}
        </span>
        <div className="flex-1 flex gap-1">
          {partitions.map(p => (
            <div
              key={p}
              className={`flex-1 h-8 rounded-md ${
                PARTITION_COLORS[p % PARTITION_COLORS.length]
              } flex items-center justify-center transition-all hover:scale-105`}
              title={`Partition ${p} → ${partitionMap.get(p)}`}
            >
              <span className="text-xs font-bold text-white">P{p}</span>
            </div>
          ))}
        </div>
      </div>
      <div className="flex gap-1 ml-44 pl-2">
        {partitions.map(p => (
          <div key={p} className="flex-1 text-center">
            <span className="text-[10px] text-gray-400 truncate block">
              {partitionMap.get(p)}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function ConsumerGroupCard({
  group,
  isExpanded,
  onToggle,
}: {
  group: ConsumerGroup
  isExpanded: boolean
  onToggle: () => void
}) {
  const stateStyle = GROUP_STATE_COLORS[group.state] || GROUP_STATE_COLORS.Empty

  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
      {/* Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-6 py-4 hover:bg-gray-50 transition-colors"
      >
        <div className="flex items-center gap-3">
          <span className="text-lg">
            {isExpanded ? '▾' : '▸'}
          </span>
          <span className="font-semibold text-gray-900 font-mono text-sm">
            {group.group_id}
          </span>
          <span
            className={`px-2 py-0.5 text-xs font-medium rounded-full border ${stateStyle.bg} ${stateStyle.text} ${stateStyle.border}`}
          >
            {group.state}
          </span>
        </div>
        <div className="flex items-center gap-4 text-sm text-gray-500">
          <span>{group.members.length} instance{group.members.length !== 1 ? 's' : ''}</span>
          <span>
            {group.members.reduce((s, m) => s + (m.partitions?.length || 0), 0)} partitions
          </span>
          <span>
            {group.members
              .reduce((s, m) => s + m.messages_read, 0)
              .toLocaleString()}{' '}
            msgs
          </span>
        </div>
      </button>

      {/* Detail table */}
      {isExpanded && (
        <div className="border-t border-gray-100 px-6 pb-4">
          <table className="w-full mt-3">
            <thead>
              <tr className="text-xs font-medium text-gray-400 uppercase tracking-wider">
                <th className="text-left pb-2">Instance</th>
                <th className="text-left pb-2">Status</th>
                <th className="text-left pb-2">Partitions</th>
                <th className="text-right pb-2">Messages</th>
                <th className="text-right pb-2">Last Activity</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {group.members.map(member => {
                const statusStyle =
                  STATUS_COLORS[member.status] || STATUS_COLORS.active
                return (
                  <tr key={member.instance_id} className="group">
                    <td className="py-2.5">
                      <span className="font-mono text-sm text-gray-800">
                        {member.instance_id}
                      </span>
                    </td>
                    <td className="py-2.5">
                      <span
                        className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${statusStyle.bg} ${statusStyle.text}`}
                      >
                        <span
                          className={`w-1.5 h-1.5 rounded-full ${statusStyle.dot}`}
                        />
                        {member.status}
                      </span>
                    </td>
                    <td className="py-2.5">
                      <div className="flex gap-1">
                        {member.partitions?.length > 0 ? (
                          member.partitions.map(p => (
                            <span
                              key={p}
                              className={`inline-flex items-center justify-center w-6 h-6 rounded text-xs font-bold text-white ${
                                PARTITION_COLORS[p % PARTITION_COLORS.length]
                              }`}
                            >
                              {p}
                            </span>
                          ))
                        ) : (
                          <span className="text-xs text-gray-300">—</span>
                        )}
                      </div>
                    </td>
                    <td className="py-2.5 text-right">
                      <span className="font-mono text-sm text-gray-700">
                        {member.messages_read.toLocaleString()}
                      </span>
                    </td>
                    <td className="py-2.5 text-right">
                      <span className="text-xs text-gray-400">
                        {member.last_message &&
                        member.last_message !== '0001-01-01T00:00:00Z'
                          ? new Date(member.last_message).toLocaleTimeString()
                          : '—'}
                      </span>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function RebalanceEventRow({ event }: { event: RebalanceEvent }) {
  const style = EVENT_TYPE_COLORS[event.event_type] || EVENT_TYPE_COLORS.joined

  return (
    <div
      className={`flex items-center gap-3 px-4 py-2.5 rounded-lg ${style.bg} transition-colors`}
    >
      <span className="text-lg w-6 text-center">{style.icon}</span>
      <span
        className={`text-xs font-semibold uppercase tracking-wider w-20 ${style.text}`}
      >
        {event.event_type}
      </span>
      <span className="font-mono text-sm text-gray-700 flex-1">
        <span className="text-gray-400">{event.group_id}</span>
        <span className="mx-1.5 text-gray-300">/</span>
        <span className="font-medium">{event.instance_id}</span>
      </span>
      {event.partitions?.length > 0 && (
        <div className="flex gap-1">
          {event.partitions.map(p => (
            <span
              key={p}
              className={`w-5 h-5 rounded text-[10px] font-bold text-white flex items-center justify-center ${
                PARTITION_COLORS[p % PARTITION_COLORS.length]
              }`}
            >
              {p}
            </span>
          ))}
        </div>
      )}
      <span className="text-xs text-gray-400 w-20 text-right">
        {new Date(event.timestamp).toLocaleTimeString()}
      </span>
    </div>
  )
}
