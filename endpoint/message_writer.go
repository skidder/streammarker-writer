package endpoint

import (
	"errors"
	"github.com/urlgrey/streammarker-writer/config"
	"github.com/urlgrey/streammarker-writer/db"

	"golang.org/x/net/context"
)

// MessageWriter provides functions for writing messages to storage
type MessageWriter interface {
	Run(context.Context, interface{}) (interface{}, error)
}

type messageWriter struct {
	db *db.Database
}

// NewMessageWriter creates a new healthcheck
func NewMessageWriter(c *config.Configuration) MessageWriter {
	return &messageWriter{c.Database}
}

func (h *messageWriter) Run(ctx context.Context, i interface{}) (interface{}, error) {
	request, ok := i.(*db.SensorReadingQueueMessage)
	if !ok {
		return nil, errors.New("Bad cast of request value")
	}

	return nil, h.db.WriteSensorReading(request)
}
