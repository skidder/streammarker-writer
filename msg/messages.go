package msg

// Measurement contains measurement details
type Measurement struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// SensorReadingQueueMessage represnets a sensor reading message sitting on the queue
type SensorReadingQueueMessage struct {
	RelayID            string        `json:"relay_id"`
	SensorID           string        `json:"sensor_id"`
	ReadingTimestamp   int32         `json:"reading_timestamp"`
	ReportingTimestamp int32         `json:"reporting_timestamp"`
	Measurements       []Measurement `json:"measurements"`
}
