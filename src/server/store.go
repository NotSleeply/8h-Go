package server

import (
	"sync"
)

type DeliveryEntry struct {
	To       string
	Acked    bool
	Retry    int
	LastSent int64
}

// InMemoryStore is a minimal in-memory store for messages and deliveries.
type InMemoryStore struct {
	mu         sync.Mutex
	nextSeq    map[string]uint64                    // chatID -> next seq
	messages   map[string]*Message                  // serverMsgID -> Message
	byClient   map[string]map[string]*Message       // from -> clientMsgID -> Message
	deliveries map[string]map[string]*DeliveryEntry // serverMsgID -> to -> DeliveryEntry
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		nextSeq:    make(map[string]uint64),
		messages:   make(map[string]*Message),
		byClient:   make(map[string]map[string]*Message),
		deliveries: make(map[string]map[string]*DeliveryEntry),
	}
}

func (s *InMemoryStore) NextSeq(chatID string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := s.nextSeq[chatID] + 1
	s.nextSeq[chatID] = seq
	return seq, nil
}

func (s *InMemoryStore) SaveMessage(msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if msg == nil || msg.ServerMsgID == "" {
		return nil
	}
	s.messages[msg.ServerMsgID] = msg
	if msg.From != "" && msg.ClientMsgID != "" {
		if _, ok := s.byClient[msg.From]; !ok {
			s.byClient[msg.From] = make(map[string]*Message)
		}
		s.byClient[msg.From][msg.ClientMsgID] = msg
	}
	return nil
}

func (s *InMemoryStore) GetMessageByClientID(from, clientMsgID string) *Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if from == "" || clientMsgID == "" {
		return nil
	}
	if m, ok := s.byClient[from]; ok {
		if msg, ok2 := m[clientMsgID]; ok2 {
			return msg
		}
	}
	return nil
}

func (s *InMemoryStore) GetMessageByServerID(serverMsgID string) *Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.messages[serverMsgID]; ok {
		return m
	}
	return nil
}

func (s *InMemoryStore) SaveDelivery(serverMsgID, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if serverMsgID == "" || to == "" {
		return nil
	}
	if _, ok := s.deliveries[serverMsgID]; !ok {
		s.deliveries[serverMsgID] = make(map[string]*DeliveryEntry)
	}
	if _, exists := s.deliveries[serverMsgID][to]; !exists {
		s.deliveries[serverMsgID][to] = &DeliveryEntry{To: to, Acked: false, Retry: 0}
	}
	return nil
}

func (s *InMemoryStore) GetRecipients(serverMsgID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []string
	if m, ok := s.deliveries[serverMsgID]; ok {
		for to, entry := range m {
			if entry != nil && !entry.Acked {
				res = append(res, to)
			}
		}
	}
	return res
}

func (s *InMemoryStore) MarkDeliveryAcked(serverMsgID, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.deliveries[serverMsgID]; ok {
		if e, ok2 := m[to]; ok2 {
			e.Acked = true
		}
	}
	return nil
}
