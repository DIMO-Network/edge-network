package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DIMO-Network/edge-network/internal/constants"
	"github.com/google/uuid"
)

type diskStorageQueue struct {
	mu     sync.Mutex
	unitID uuid.UUID
}

func NewDiskStorageQueue(unitID uuid.UUID) StorageQueue {
	return &diskStorageQueue{unitID: unitID}
}

func (s *diskStorageQueue) Dequeue() ([]Message, error) {
	files, err := os.ReadDir(constants.TmpDirectory)
	if err != nil {
		return nil, err
	}

	var messages []Message
	if len(files) > 0 {
		var enqueueFiles []os.FileInfo
		for _, file := range files {
			if strings.HasPrefix(file.Name(), messagePrefix) {
				file, _ := os.Stat(filepath.Join(constants.TmpDirectory, file.Name()))
				enqueueFiles = append(enqueueFiles, file)
			}
		}

		sort.Slice(enqueueFiles, func(i, j int) bool {
			return enqueueFiles[i].ModTime().Before(enqueueFiles[j].ModTime())
		})

		top := 10
		for i := 0; i < top && i < len(enqueueFiles); i++ {
			file := enqueueFiles[i]
			filePath := filepath.Join(constants.TmpDirectory, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}

			var message Message
			err = json.Unmarshal(data, &message)
			if err != nil {
				return nil, err
			}

			err = os.Remove(filePath)
			if err != nil {
				return nil, err
			}

			messages = append(messages, message)
		}
	}

	return messages, nil
}

func (s *diskStorageQueue) Enqueue(message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentTime := time.Now()
	messageObj := Message{
		Time:    currentTime,
		Content: message,
	}

	data, err := json.Marshal(messageObj)
	if err != nil {
		return err
	}

	fileName := currentTime.Format("2006-01-02_15-04-05.json")
	filePath := filepath.Join(constants.TmpDirectory, messagePrefix+fileName)

	// Open the file for writing (create if it doesn't exist)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %s", err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("error writing file: %s", err)
	}
	return nil
}
