package queue

//go:generate mockgen -source storage_queue.go -destination mocks/storage_queue_mock.go

type StorageQueue interface {
	Enqueue(message string) error
	Dequeue() ([]Message, error)
}
