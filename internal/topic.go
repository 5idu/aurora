package internal

import (
	"math/rand"
	"sync/atomic"
	"time"
)

type Topic struct {
	Name       string
	Partitions []*Partition
}

type Partition struct {
	Name   string
	broker ServerInfo
}

type Message struct {
	// Payload for the message
	Payload []byte
	// Key sets the key of the message for routing policy
	Key string
	// OrderingKey sets the ordering key of the message
	OrderingKey string
	// Properties attach application defined properties on the message
	Properties map[string]string
	// message create time
	CreatedAt time.Time
	// SequenceID set the sequence id to assign to the current message
	SequenceID *int64
	// Request to deliver the message only after the specified relative delay.
	DeliverAfter time.Duration
	// Deliver the message only at or after the specified absolute timestamp.
	DeliverAt time.Time
}

type RoutingStrategy interface {
	Routing(topic *Topic, message *Message) *Partition
}

type RoundRobinStrategy struct {
	next uint32
}

func (r *RoundRobinStrategy) Routing(topic *Topic, message *Message) *Partition {
	if len(topic.Partitions) == 0 {
		return nil
	}
	if message.Key == "" {
		n := atomic.AddUint32(&r.next, 1)
		return topic.Partitions[(int(n)-1)%len(topic.Partitions)]
	}
	// hash routing
	return nil
}

type RandomStrategy struct{}

func (r *RandomStrategy) Routing(topic *Topic, message *Message) *Partition {
	if len(topic.Partitions) == 0 {
		return nil
	}
	if message.Key == "" {
		i := rand.Intn(len(topic.Partitions))
		return topic.Partitions[i]
	}

	// hash routing
	return nil
}
