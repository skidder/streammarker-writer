package endpoint

import (
	"errors"
	"github.com/urlgrey/streammarker-writer/config"
	"github.com/urlgrey/streammarker-writer/db"
	"github.com/urlgrey/streammarker-writer/msg"

	"golang.org/x/net/context"
)

// MessageWriter provides functions for writing messages to storage
type MessageWriter interface {
	Run(context.Context, interface{}) (interface{}, error)
}

type messageWriter struct {
	measurementWriter db.MeasurementWriter
	deviceManager     db.DeviceManager
}

// NewMessageWriter creates a new healthcheck
func NewMessageWriter(c *config.Configuration) MessageWriter {
	return &messageWriter{c.MeasurementWriter, c.DeviceManager}
}

func (h *messageWriter) Run(ctx context.Context, i interface{}) (interface{}, error) {
	request, ok := i.(*msg.SensorReadingQueueMessage)
	if !ok {
		return nil, errors.New("Bad cast of request value")
	}

	if err := h.measurementWriter.WriteSensorReading(request); err != nil {
		return nil, err
	}
	return nil, nil
}
