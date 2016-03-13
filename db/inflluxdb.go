package db

import (
	"errors"
	"log"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/urlgrey/streammarker-writer/msg"
)

const (
	defaultDatabase = "streammarker_measurements"
)

// InfluxDAO represents a DAO capable of interacting with InfluxDB
type InfluxDAO struct {
	c             client.Client
	deviceManager DeviceManager
}

// NewInfluxDAO creates a new DAO for interacting with InfluxDB
func NewInfluxDAO(address string, username string, password string, deviceManager DeviceManager) (*InfluxDAO, error) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     address,
		Username: username,
		Password: password,
	})
	return &InfluxDAO{c, deviceManager}, err
}

// WriteSensorReading will record the Sensor Reading data, first verifying that a corresponding reporting
// device and account exist and are active
func (i *InfluxDAO) WriteSensorReading(r *msg.SensorReadingQueueMessage) error {
	var err error
	if len(r.Measurements) == 0 {
		err = errors.New("No measurements provided in message, ignoring")
		return err
	}

	var relay *Relay
	if relay, err = i.deviceManager.GetRelay(r.RelayID); err != nil {
		return err
	}

	if !relay.isActive() {
		err = errors.New("Reporting device is not active, will not record sensor reading")
		return err
	}

	var sensor *Sensor
	if sensor, err = i.deviceManager.GetSensor(r.SensorID, relay.AccountID); err != nil {
		return err
	}

	// if the sensor doesn't exist, then create it and associate with the relay account
	if sensor == nil {
		log.Printf("Sensor not found, adding: %s", r.SensorID)
		if sensor, err = i.deviceManager.CreateSensor(r.SensorID, relay.AccountID); err != nil {
			return err
		}
	} else {
		if relay.AccountID != sensor.AccountID {
			log.Printf("Sensor and Relay use different account IDs, ignoring: sensor account=%s, relay account=%s", sensor.AccountID, relay.AccountID)
			err = errors.New("Sensor and Relay use different account IDs, ignoring")
			return err
		}
	}

	// Create a new point batch
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  defaultDatabase,
		Precision: "s",
	})
	if err != nil {
		return err
	}

	for _, m := range r.Measurements {
		// Create a point and add to batch
		tags := map[string]string{
			"account_id": sensor.AccountID,
			"sensor_id":  sensor.ID,
			"relay_id":   relay.ID,
		}
		fields := map[string]interface{}{
			"value": m.Value,
			"unit":  m.Unit,
		}
		if sensor.LocationEnabled && sensor.Latitude != 0 && sensor.Longitude != 0 {
			fields["latitude"] = sensor.Latitude
			fields["longitude"] = sensor.Longitude
		}
		var pt *client.Point
		pt, err = client.NewPoint(m.Name, tags, fields, time.Unix(int64(r.ReadingTimestamp), 0))
		if err != nil {
			return err
		}
		bp.AddPoint(pt)
	}

	// Write the batch
	return i.c.Write(bp)
}
