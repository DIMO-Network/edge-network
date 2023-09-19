package queue

import "time"

const (
	messagePrefix = "msg_"
)

type Message struct {
	Time    time.Time `json:"time"`
	Content string    `json:"content"`
}
