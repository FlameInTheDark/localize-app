package operations

import (
	"sync"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type Emitter func(domain.OperationProgress)

type Hub struct {
	mu      sync.RWMutex
	emitter Emitter
}

func (h *Hub) SetEmitter(emitter Emitter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.emitter = emitter
}

func (h *Hub) Emit(progress domain.OperationProgress) {
	h.mu.RLock()
	emitter := h.emitter
	h.mu.RUnlock()
	if emitter != nil {
		emitter(progress)
	}
}
