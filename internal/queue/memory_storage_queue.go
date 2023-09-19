package queue

import (
	"github.com/google/uuid"
	"sync"
	"time"
)

type memoryStorageQueue struct {
	mu     sync.Mutex
	queues []Message
	unitID uuid.UUID
}

func NewMemoryStorageQueue(unitID uuid.UUID) StorageQueue {
	return &memoryStorageQueue{
		unitID: unitID,
		queues: make([]Message, 0),
	}
}

func (s *memoryStorageQueue) Dequeue() ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queues) == 0 {
		return nil, nil
	}
	var messages []Message
	top := 10
	if len(s.queues) < top {
		messages = make([]Message, len(s.queues))
	} else {
		messages = make([]Message, top)
	}
	copy(messages, s.queues[:len(messages)])
	s.queues = s.queues[len(messages):]
	return messages, nil
}

func (s *memoryStorageQueue) Enqueue(message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentTime := time.Now()
	messageObj := Message{
		Time:    currentTime,
		Content: message,
	}

	s.queues = append(s.queues, messageObj)

	return nil
}
