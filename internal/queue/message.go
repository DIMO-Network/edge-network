package queue

import "time"

const (
	messagePrefix = "msg_"
)

type Message struct {
	Name    string    `json:"name"`
	Time    time.Time `json:"time"`
	Content any       `json:"content"`
}
