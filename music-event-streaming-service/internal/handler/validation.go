package handler

import (
	"context"

	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/consumer"
)

type Validation struct {
	name string
}

func NewValidation(name string) *Validation {
	return &Validation{name: name}
}

func (h *Validation) Name() string {
	return h.name
}

func (*Validation) Handle(context.Context, consumer.Message) error {
	return nil
}
