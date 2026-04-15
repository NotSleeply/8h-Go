package store

import (
	"sync"

	"tet/src/protocol"
	"tet/src/storage"
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

// InMemoryStore is a hybrid store:
// - fast-path cache in memory
// - optional persistence in MySQL when storage.DB is initialized
type InMemoryStore struct {
	mu         sync.Mutex
	nextSeq    map[string]uint64                       // chatID -> next seq
	messages   map[string]*protocol.Message            // serverMsgID -> Message
	byClient   map[string]map[string]*protocol.Message // from -> clientMsgID -> Message
	deliveries map[string]map[string]*DeliveryEntry    // serverMsgID -> to -> DeliveryEntry

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
