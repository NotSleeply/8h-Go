package store

import (
	"sync"

	"goim/internal/protocol"
	"goim/internal/storage"
)

type DeliveryEntry struct {
	To          string
	Acked       bool
	Read        bool
	Dead        bool
	Retry       int
	LastSent    int64
	NextRetryAt int64
	LastError   string
}

type InMemoryStore struct {
	mu         sync.Mutex
	nextSeq    map[string]uint64
	messages   map[string]*protocol.Message
	byClient   map[string]map[string]*protocol.Message
	deliveries map[string]map[string]*DeliveryEntry

	persistent bool
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		nextSeq:    make(map[string]uint64),
		messages:   make(map[string]*protocol.Message),
		byClient:   make(map[string]map[string]*protocol.Message),
		deliveries: make(map[string]map[string]*DeliveryEntry),
		persistent: storage.DB != nil,
	}
}
