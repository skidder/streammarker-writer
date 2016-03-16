package db

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/urlgrey/streammarker-writer/msg"
)

// InfluxDAO represents a DAO capable of interacting with InfluxDB
type InfluxDAO struct {
	c             client.Client
	deviceManager DeviceManager
	databaseName  string
}

// NewInfluxDAO creates a new DAO for interacting with InfluxDB
func NewInfluxDAO(address string, username string, password string, databaseName string, deviceManager DeviceManager) (*InfluxDAO, error) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     address,
		Username: username,
		Password: password,
	})
	return &InfluxDAO{c, deviceManager, databaseName}, err
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
		Database:  i.databaseName,
		Precision: "s",
	})
	if err != nil {
		return err
	}

	readingTimestamp := time.Unix(int64(r.ReadingTimestamp), int64(0))
	for _, m := range r.Measurements {
		if i.shouldEvaluateSensorReading(&readingTimestamp, sensor, m.Name) == false {
			continue
		}

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
		pt, err = client.NewPoint(m.Name, tags, fields, readingTimestamp)
		if err != nil {
			return err
		}
		bp.AddPoint(pt)
	}

	if len(bp.Points()) == 0 {
		// don't attempt to write to database if there are no points
		return nil
	}

	// Write the batch
	return i.c.Write(bp)
}

// queryDB convenience function to query the database
func (i *InfluxDAO) queryDB(cmd string) (res []client.Result, err error) {
	q := client.Query{
		Command:  cmd,
		Database: i.databaseName,
	}
	if response, err := i.c.Query(q); err == nil {
		if response.Error() != nil {
			return res, response.Error()
		}
		res = response.Results
	} else {
		return res, err
	}
	return res, nil
}

func (i *InfluxDAO) getTimeOfLastReadingForSensor(sensorID string, accountID string, measurementName string, timestamp *time.Time) (*time.Time, error) {
	res, err := i.queryDB(fmt.Sprintf("SELECT * from %s where sensor_id = '%s' and account_id = '%s' order by time desc limit 1", measurementName, sensorID, accountID))
	if err != nil {
		return nil, err
	}
	if len(res) != 1 || res[0].Series == nil {
		// the query returned no rows, must be empty
		return nil, nil
	}
	row := res[0].Series[0].Values[0]
	var t time.Time
	t, err = time.Parse(time.RFC3339, row[0].(string))
	return &t, err
}

func (i *InfluxDAO) shouldEvaluateSensorReading(readingTimestamp *time.Time, sensor *Sensor, measurementName string) bool {
	var lastReadingTimestamp *time.Time
	var err error
	if lastReadingTimestamp, err = i.getTimeOfLastReadingForSensor(sensor.ID, sensor.AccountID, measurementName, readingTimestamp); err != nil {
		log.Printf("Error while looking up timestamp of last reading for sensor, proceeding anyway: Sensor ID=%s, Error=%s", sensor.ID, err.Error())
		return true
	}

	if lastReadingTimestamp != nil {
		log.Printf("Evaluating timestamp: %d", readingTimestamp.Unix())
		secondsElapsed := readingTimestamp.Sub(*lastReadingTimestamp).Seconds()
		sampleFrequency := sensor.SampleFrequency
		log.Printf("Seconds since last reading was written: %d", int32(secondsElapsed))
		if secondsElapsed < float64(sampleFrequency-sampleFrequencyTolerance) {
			log.Printf("Ignoring reading for sensor %s due to sample frequency limit (%d seconds)", sensor.ID, sampleFrequency)
			return false
		}
	}
	return true
}
