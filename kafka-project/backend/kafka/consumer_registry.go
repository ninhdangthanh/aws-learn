package kafka

import (
	"sync"
	"time"
)

// ConsumerInstanceInfo tracks a single consumer instance
type ConsumerInstanceInfo struct {
	GroupID      string    `json:"group_id"`
	InstanceID   string    `json:"instance_id"`
	Status       string    `json:"status"` // "active", "stopped", "rebalancing"
	Partitions   []int     `json:"partitions"`
	MessagesRead int64     `json:"messages_read"`
	LastMessage  time.Time `json:"last_message"`
	StartedAt    time.Time `json:"started_at"`
}

// RebalanceEvent records when partitions are reassigned
type RebalanceEvent struct {
	GroupID    string    `json:"group_id"`
	InstanceID string   `json:"instance_id"`
	EventType  string   `json:"event_type"` // "assigned", "revoked", "joined", "left"
	Partitions []int    `json:"partitions"`
	Timestamp  time.Time `json:"timestamp"`
}

// ConsumerGroupStatus aggregated status for a consumer group
type ConsumerGroupStatus struct {
	GroupID   string                 `json:"group_id"`
	Members  []ConsumerInstanceInfo `json:"members"`
	State    string                 `json:"state"` // "Stable", "Rebalancing", "Empty"
}

// Registry is the global thread-safe consumer registry
type Registry struct {
	mu              sync.RWMutex
	instances       map[string]*ConsumerInstanceInfo // key: "groupID:instanceID"
	rebalanceEvents []RebalanceEvent
	maxEvents       int
}

var (
	globalRegistry *Registry
	registryOnce   sync.Once
)

// GetRegistry returns the singleton consumer registry
func GetRegistry() *Registry {
	registryOnce.Do(func() {
		globalRegistry = &Registry{
			instances:       make(map[string]*ConsumerInstanceInfo),
			rebalanceEvents: make([]RebalanceEvent, 0),
			maxEvents:       200,
		}
	})
	return globalRegistry
}

func registryKey(groupID, instanceID string) string {
	return groupID + ":" + instanceID
}

// Register adds a consumer instance to the registry
func (r *Registry) Register(groupID, instanceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := registryKey(groupID, instanceID)
	r.instances[key] = &ConsumerInstanceInfo{
		GroupID:    groupID,
		InstanceID: instanceID,
		Status:     "active",
		Partitions: []int{},
		StartedAt:  time.Now(),
	}

	r.addEvent(RebalanceEvent{
		GroupID:    groupID,
		InstanceID: instanceID,
		EventType:  "joined",
		Partitions: []int{},
		Timestamp:  time.Now(),
	})
}

// Unregister removes a consumer instance
func (r *Registry) Unregister(groupID, instanceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := registryKey(groupID, instanceID)
	if info, ok := r.instances[key]; ok {
		r.addEvent(RebalanceEvent{
			GroupID:    groupID,
			InstanceID: instanceID,
			EventType:  "left",
			Partitions: info.Partitions,
			Timestamp:  time.Now(),
		})
		delete(r.instances, key)
	}
}

// UpdatePartitions updates the partition assignment for an instance
func (r *Registry) UpdatePartitions(groupID, instanceID string, partitions []int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := registryKey(groupID, instanceID)
	if info, ok := r.instances[key]; ok {
		oldPartitions := info.Partitions
		info.Partitions = partitions
		info.Status = "active"

		// Only record rebalance event if partitions actually changed
		if !intSliceEqual(oldPartitions, partitions) {
			if len(oldPartitions) > 0 {
				r.addEvent(RebalanceEvent{
					GroupID:    groupID,
					InstanceID: instanceID,
					EventType:  "revoked",
					Partitions: oldPartitions,
					Timestamp:  time.Now(),
				})
			}
			r.addEvent(RebalanceEvent{
				GroupID:    groupID,
				InstanceID: instanceID,
				EventType:  "assigned",
				Partitions: partitions,
				Timestamp:  time.Now(),
			})
		}
	}
}

// RecordMessage records that a message was processed
func (r *Registry) RecordMessage(groupID, instanceID string, partition int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := registryKey(groupID, instanceID)
	if info, ok := r.instances[key]; ok {
		info.MessagesRead++
		info.LastMessage = time.Now()

		// Track partition if not already tracked
		found := false
		for _, p := range info.Partitions {
			if p == partition {
				found = true
				break
			}
		}
		if !found {
			info.Partitions = append(info.Partitions, partition)
		}
	}
}

// SetStatus updates the status of a consumer instance
func (r *Registry) SetStatus(groupID, instanceID, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := registryKey(groupID, instanceID)
	if info, ok := r.instances[key]; ok {
		info.Status = status
	}
}

// GetGroupStatus returns aggregated status for a consumer group
func (r *Registry) GetGroupStatus(groupID string) ConsumerGroupStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := ConsumerGroupStatus{
		GroupID: groupID,
		Members: []ConsumerInstanceInfo{},
		State:   "Empty",
	}

	for _, info := range r.instances {
		if info.GroupID == groupID {
			status.Members = append(status.Members, *info)
		}
	}

	if len(status.Members) > 0 {
		status.State = "Stable"
		for _, m := range status.Members {
			if m.Status == "rebalancing" {
				status.State = "Rebalancing"
				break
			}
		}
	}

	return status
}

// GetAllGroupStatuses returns status for all consumer groups
func (r *Registry) GetAllGroupStatuses() []ConsumerGroupStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groupMap := make(map[string]*ConsumerGroupStatus)

	for _, info := range r.instances {
		gs, ok := groupMap[info.GroupID]
		if !ok {
			gs = &ConsumerGroupStatus{
				GroupID: info.GroupID,
				Members: []ConsumerInstanceInfo{},
				State:   "Stable",
			}
			groupMap[info.GroupID] = gs
		}
		gs.Members = append(gs.Members, *info)
		if info.Status == "rebalancing" {
			gs.State = "Rebalancing"
		}
	}

	result := make([]ConsumerGroupStatus, 0, len(groupMap))
	for _, gs := range groupMap {
		if len(gs.Members) == 0 {
			gs.State = "Empty"
		}
		result = append(result, *gs)
	}

	return result
}

// GetRebalanceEvents returns the most recent rebalance events
func (r *Registry) GetRebalanceEvents(limit int) []RebalanceEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > len(r.rebalanceEvents) {
		limit = len(r.rebalanceEvents)
	}

	// Return most recent events (end of the slice)
	start := len(r.rebalanceEvents) - limit
	result := make([]RebalanceEvent, limit)
	copy(result, r.rebalanceEvents[start:])

	// Reverse so newest is first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// addEvent appends a rebalance event, evicting old ones if necessary
func (r *Registry) addEvent(event RebalanceEvent) {
	r.rebalanceEvents = append(r.rebalanceEvents, event)
	if len(r.rebalanceEvents) > r.maxEvents {
		r.rebalanceEvents = r.rebalanceEvents[len(r.rebalanceEvents)-r.maxEvents:]
	}
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[int]bool)
	for _, v := range a {
		m[v] = true
	}
	for _, v := range b {
		if !m[v] {
			return false
		}
	}
	return true
}
