package db

import (
	"github.com/urlgrey/streammarker-writer/msg"
)

// MeasurementWriter is capable of writing sensor readings to storage
type MeasurementWriter interface {
	WriteSensorReading(*msg.SensorReadingQueueMessage) error
}

// DeviceManager allows for storage & retrieval of devices (sensors & relays)
type DeviceManager interface {
	GetRelay(string) (*Relay, error)
	GetSensor(string, string) (*Sensor, error)
	CreateSensor(string, string) (*Sensor, error)
}
