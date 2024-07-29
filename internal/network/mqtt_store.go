package network

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"sync"
)

// CustomFileStore is a custom implementation of the mqtt.FileStore interface.
// It extends the FileStore tomyFileStore.
// The 'counter' field keeps track of the number of keys currently in the store.
// The 'limit' field is the maximum number of keys that the store can hold.
// The 'isRead' field is a flag used to ensure that the counter is only set to the current number of keys in the store once.
type CustomFileStore struct {
	*mqtt.FileStore
	counter int
	limit   int
	isRead  bool
	mu      sync.Mutex
}

// Put adds a new key-value pair to the store. The key is a string and the value is a ControlPacket.
// It first checks if the store has reached its limit. If it has, it prints "key limit reached" and does not add the new key-value pair.
// If the store has not reached its limit, it adds the new key-value pair and increments the counter.
// The counter keeps track of the number of keys in the store.
// The limit is the maximum number of keys that the store can hold.
// The isRead flag is used to ensure that the counter is only set to the current number of keys in the store once.
func (store *CustomFileStore) Put(key string, m packets.ControlPacket) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if !store.isRead {
		store.counter = len(store.All())
		store.isRead = true
	}

	if store.counter < store.limit {
		store.FileStore.Put(key, m)
		store.counter++
	} else {
		fmt.Println("key limit reached")
	}
}
