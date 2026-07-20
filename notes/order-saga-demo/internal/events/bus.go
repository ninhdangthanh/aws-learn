package events

import "sync"

type Handler func(Event)

type Bus struct {
	mu       sync.Mutex
	handlers map[string][]Handler
}

func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]Handler)}
}

func (b *Bus) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
}

// Publish copies the handler slice under the lock, then releases the lock
// before invoking handlers — this lets a handler call Publish again (as the
// saga does) without deadlocking on the same Bus.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	hs := append([]Handler(nil), b.handlers[e.Name]...)
	b.mu.Unlock()

	for _, h := range hs {
		h(e)
	}
}
