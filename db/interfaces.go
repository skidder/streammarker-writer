package db

import (
	"github.com/urlgrey/streammarker-writer/msg"
)

// Database is capable of writing sensor readings to storage
type Database interface {
	WriteSensorReading(*msg.SensorReadingQueueMessage) error
}
