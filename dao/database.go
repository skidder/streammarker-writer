package dao

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

const (
	TABLE_TIMESTAMP_FORMAT        = "2006-01"
	HOURLY_TABLE_TIMESTAMP_FORMAT = "2006-01"
	SAMPLE_FREQUENCY_TOLERANCE    = 10
)

type Database struct {
	dynamoDBService *dynamodb.DynamoDB
}

func NewDatabase(dynamoDBService *dynamodb.DynamoDB) *Database {
	return &Database{dynamoDBService: dynamoDBService}
}

// Record the Sensor Reading data, first verifying that a corresponding reporting
// device and account exist and are active
func (d *Database) WriteSensorReading(r *SensorReadingQueueMessage) error {
	var err error
	if len(r.Measurements) == 0 {
		err = errors.New("No measurements provided in message, ignoring")
		return err
	}

	var relay *Relay
	if relay, err = d.GetRelay(r.RelayID); err != nil {
		return err
	}

	if !relay.isActive() {
		err = errors.New("Reporting device is not active, will not record sensor reading")
		return err
	}

	var sensor *Sensor
	if sensor, err = d.getSensor(r.SensorID, relay.AccountID); err != nil {
		return err
	}

	// if the sensor doesn't exist, then create it and associate with the relay account
	if sensor == nil {
		log.Printf("Sensor not found, adding: %s", r.SensorID)
		if sensor, err = d.createSensor(r.SensorID, relay.AccountID); err != nil {
			return err
		}
	} else {
		if relay.AccountID != sensor.AccountID {
			log.Printf("Sensor and Relay use different account IDs, ignoring: sensor account=%s, relay account=%s", sensor.AccountID, relay.AccountID)
			err = errors.New("Sensor and Relay use different account IDs, ignoring")
			return err
		}
	}

	// check whether the sample frequency indicates we should ignore this reading
	readingTimestamp := time.Unix(int64(r.ReadingTimestamp), 0)
	if !d.shouldEvaluateSensorReading(&readingTimestamp, sensor) {
		return err
	}

	// Write measurements to database
	if err = d.recordMeasurement(r, sensor, &readingTimestamp); err != nil {
		return err
	}
	err = d.recordHourlyMeasurement(r, sensor, &readingTimestamp)
	return err
}

func (d *Database) recordHourlyMeasurement(r *SensorReadingQueueMessage, sensor *Sensor, readingTimestamp *time.Time) error {
	var err error
	hourlySensorReadingsTableName := fmt.Sprintf("hourly_sensor_readings_%s", readingTimestamp.Format(HOURLY_TABLE_TIMESTAMP_FORMAT))
	hourlyRoundedTimestamp := time.Date(readingTimestamp.Year(), readingTimestamp.Month(), readingTimestamp.Day(), readingTimestamp.Hour(), 0, 0, 0, readingTimestamp.Location())

	// query the table for an existing hourly record
	params := &dynamodb.QueryInput{
		TableName: aws.String(hourlySensorReadingsTableName),
		AttributesToGet: []*string{
			aws.String("measurements"),
		},
		ScanIndexForward: aws.Bool(false),
		KeyConditions: map[string]*dynamodb.Condition{
			"id": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(fmt.Sprintf("%s:%s", sensor.AccountID, sensor.ID)),
					},
				},
			},
			"timestamp": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						N: aws.String(fmt.Sprintf("%d", hourlyRoundedTimestamp.Unix())),
					},
				},
			},
		},
		Limit: aws.Int64(1),
	}

	var measurements []MinMaxMeasurement
	measurements = nil
	var resp *dynamodb.QueryOutput
	if resp, err = d.dynamoDBService.Query(params); err == nil {
		for _, sensorRecord := range resp.Items {
			if err = json.Unmarshal([]byte(*sensorRecord["measurements"].S), &measurements); err != nil {
				return err
			}
		}
	} else {
		// check whether error is due to the table being missing, in which case we should create it
		listTablesInput := &dynamodb.ListTablesInput{
			ExclusiveStartTableName: aws.String(hourlySensorReadingsTableName),
			Limit: aws.Int64(1),
		}
		var resp *dynamodb.ListTablesOutput
		if resp, err = d.dynamoDBService.ListTables(listTablesInput); err == nil {
			if (resp.LastEvaluatedTableName == nil) || (*resp.LastEvaluatedTableName != hourlySensorReadingsTableName) {
				// table doesn't exist, let's make it
				log.Printf("Table doesn't exist, will create it: %s", hourlySensorReadingsTableName)
				if err = d.createSensorReadingsTable(hourlySensorReadingsTableName); err == nil {
					log.Println("Created table, attempting to put the hourly sensor reading again")
				}
			} else {
				log.Println("Table exists, will wait in case the hourly sensor readings table is being created")
				time.Sleep(d.getTableWaitTime())
			}
		}
	}
	// if no record was returned or table had to be created
	// build new record to store with sensor reading values as the min/max
	minMaxRequiresSave := false
	var mergedMinMax []MinMaxMeasurement
	if measurements == nil {
		minMaxRequiresSave = true
		// creating the table from scratch, so this reading must be min/max
		mergedMinMax = make([]MinMaxMeasurement, 0)
		for _, m := range r.Measurements {
			mergedMinMax = append(mergedMinMax, MinMaxMeasurement{
				Name: m.Name,
				Min:  m,
				Max:  m,
			})
		}
	} else {
		// else (record was returned)
		mergedMinMax = make([]MinMaxMeasurement, 0)

		for _, minMaxReading := range measurements {
			found := false
			for _, m := range r.Measurements {
				if minMaxReading.Name == m.Name {
					found = true
					break
				}
			}
			// an existing min-max measurement was not present, add it to the set
			if !found {
				mergedMinMax = append(mergedMinMax, minMaxReading)
			}
		}

		for _, m := range r.Measurements {
			found := false
			for _, minMaxReading := range measurements {
				log.Println("Comparing min-max readings")
				if minMaxReading.Name == m.Name {
					log.Println("Found match on name")
					var mergedMin, mergedMax float64
					if minMaxReading.Min.Value > m.Value {
						minMaxRequiresSave = true
						mergedMin = m.Value
					} else {
						mergedMin = minMaxReading.Min.Value
					}

					if minMaxReading.Max.Value < m.Value {
						minMaxRequiresSave = true
						mergedMax = m.Value
					} else {
						mergedMax = minMaxReading.Max.Value
					}

					mergedMinMax = append(mergedMinMax, MinMaxMeasurement{
						Name: m.Name,
						Min:  Measurement{Name: m.Name, Unit: m.Unit, Value: mergedMin},
						Max:  Measurement{Name: m.Name, Unit: m.Unit, Value: mergedMax},
					})
					found = true
					break
				}
			}

			if !found {
				// this reading wasn't present in previous submissions, add it
				minMaxRequiresSave = true
				mergedMinMax = append(mergedMinMax, MinMaxMeasurement{
					Name: m.Name,
					Min:  m,
					Max:  m,
				})
			}
		}
	}

	// store record
	if minMaxRequiresSave {
		var hourlyMeasurementsJson []byte
		if hourlyMeasurementsJson, err = json.Marshal(mergedMinMax); err != nil {
			return err
		}
		log.Println("Measurements JSON", string(hourlyMeasurementsJson))
		input := &dynamodb.PutItemInput{
			Item: map[string]*dynamodb.AttributeValue{
				"id": &dynamodb.AttributeValue{
					S: aws.String(fmt.Sprintf("%s:%s", sensor.AccountID, sensor.ID)),
				},
				"timestamp": &dynamodb.AttributeValue{
					N: aws.String(fmt.Sprintf("%d", hourlyRoundedTimestamp.Unix())),
				},
				"account_id": &dynamodb.AttributeValue{
					S: aws.String(sensor.AccountID),
				},
				"sensor_id": &dynamodb.AttributeValue{
					S: aws.String(sensor.ID),
				},
				"measurements": &dynamodb.AttributeValue{
					S: aws.String(string(hourlyMeasurementsJson)),
				},
			},
			TableName: aws.String(hourlySensorReadingsTableName),
		}

		if _, err := d.dynamoDBService.PutItem(input); err != nil {
			log.Println("Error while saving hourly measurement", err.Error())
		}
	}
	return err
}

func (d *Database) recordMeasurement(r *SensorReadingQueueMessage, sensor *Sensor, readingTimestamp *time.Time) error {
	var err error
	var measurementsJson []byte
	if measurementsJson, err = json.Marshal(r.Measurements); err != nil {
		return err
	}
	sensorReadingsTableName := fmt.Sprintf("sensor_readings_%s", readingTimestamp.Format(TABLE_TIMESTAMP_FORMAT))
	item := map[string]*dynamodb.AttributeValue{
		"id": &dynamodb.AttributeValue{
			S: aws.String(fmt.Sprintf("%s:%s", sensor.AccountID, sensor.ID)),
		},
		"timestamp": &dynamodb.AttributeValue{
			N: aws.String(fmt.Sprintf("%d", r.ReadingTimestamp)),
		},
		"account_id": &dynamodb.AttributeValue{
			S: aws.String(sensor.AccountID),
		},
		"relay_id": &dynamodb.AttributeValue{
			S: aws.String(r.RelayID),
		},
		"sensor_id": &dynamodb.AttributeValue{
			S: aws.String(sensor.ID),
		},
		"measurements": &dynamodb.AttributeValue{
			S: aws.String(string(measurementsJson)),
		},
	}
	if sensor.LocationEnabled && sensor.Latitude != 0 && sensor.Longitude != 0 {
		item["latitude"] = &dynamodb.AttributeValue{
			N: aws.String(fmt.Sprintf("%f", sensor.Latitude)),
		}
		item["longitude"] = &dynamodb.AttributeValue{
			N: aws.String(fmt.Sprintf("%f", sensor.Longitude)),
		}
	}
	input := &dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(sensorReadingsTableName),
	}

	if _, err = d.dynamoDBService.PutItem(input); err != nil {
		log.Printf("Encountered error while recording metric: %s\n", err.Error())

		// check whether error is due to the table being missing, in which case we should create it
		listTablesInput := &dynamodb.ListTablesInput{
			ExclusiveStartTableName: aws.String(sensorReadingsTableName),
			Limit: aws.Int64(1),
		}
		var resp *dynamodb.ListTablesOutput
		if resp, err = d.dynamoDBService.ListTables(listTablesInput); err == nil {
			if (resp.LastEvaluatedTableName == nil) || (*resp.LastEvaluatedTableName != sensorReadingsTableName) {
				// table doesn't exist, let's make it
				log.Printf("Table doesn't exist, will create it: %s", sensorReadingsTableName)
				if err = d.createSensorReadingsTable(sensorReadingsTableName); err == nil {
					log.Println("Created table, attempting to put the sensor reading again")
					_, err = d.dynamoDBService.PutItem(input)
				}
			} else {
				log.Println("Table exists, will wait in case the table is being created")
				time.Sleep(d.getTableWaitTime())
				_, err = d.dynamoDBService.PutItem(input)
			}
		}
	}
	return err
}

// Get the amount of time to wait for a table to finish being created
func (d *Database) getTableWaitTime() (t time.Duration) {
	var waitTime string
	if waitTime = os.Getenv("STREAMMARKER_DYNAMO_WAIT_TIME"); waitTime == "" {
		waitTime = "30s"
	}

	var err error
	if t, err = time.ParseDuration(waitTime); err != nil {
		t = 30 * time.Second
	}
	return
}

// Create a sensor-readings table with the provided table name
func (d *Database) createSensorReadingsTable(tableName string) (err error) {
	createTableInput := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{ // Required
			&dynamodb.AttributeDefinition{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"),
			},
			&dynamodb.AttributeDefinition{
				AttributeName: aws.String("timestamp"),
				AttributeType: aws.String("N"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			&dynamodb.KeySchemaElement{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("HASH"),
			},
			&dynamodb.KeySchemaElement{
				AttributeName: aws.String("timestamp"),
				KeyType:       aws.String("RANGE"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{ // Required
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(1),
		},
		TableName: aws.String(tableName),
	}
	if _, err = d.dynamoDBService.CreateTable(createTableInput); err == nil {
		log.Println("Waiting to allow table to be created")
		time.Sleep(d.getTableWaitTime())
		log.Println("Finished waiting, will resume processing")
	}
	return
}

// Get the Relay record for the given relay ID
func (d *Database) GetRelay(relayID string) (relay *Relay, err error) {
	params := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": &dynamodb.AttributeValue{
				S: aws.String(relayID),
			},
		},
		TableName: aws.String("relays"),
		AttributesToGet: []*string{
			aws.String("account_id"),
			aws.String("name"),
			aws.String("state"),
		},
		ConsistentRead: aws.Bool(true),
	}

	var resp *dynamodb.GetItemOutput
	if resp, err = d.dynamoDBService.GetItem(params); err == nil {
		if resp.Item != nil {
			relay = &Relay{
				ID:        relayID,
				AccountID: *resp.Item["account_id"].S,
				Name:      *resp.Item["name"].S,
				State:     *resp.Item["state"].S,
			}
		} else {
			err = errors.New(fmt.Sprintf("Relay not found: %s", relayID))
		}
	}
	return
}

// Get the Sensor record for the given reporting device ID
func (d *Database) getSensor(sensorId string, accountId string) (*Sensor, error) {
	params := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": &dynamodb.AttributeValue{
				S: aws.String(sensorId),
			},
		},
		TableName: aws.String("sensors"),
		AttributesToGet: []*string{
			aws.String("name"),
			aws.String("state"),
			aws.String("account_id"),
			aws.String("sample_frequency"),
			aws.String("location_enabled"),
			aws.String("latitude"),
			aws.String("longitude"),
		},
	}

	var resp *dynamodb.GetItemOutput
	var sensor *Sensor
	var err error
	if resp, err = d.dynamoDBService.GetItem(params); err == nil {
		if resp.Item != nil {
			sensor = &Sensor{
				ID:              sensorId,
				AccountID:       *resp.Item["account_id"].S,
				Name:            *resp.Item["name"].S,
				State:           *resp.Item["state"].S,
				LocationEnabled: *resp.Item["location_enabled"].BOOL,
			}
			if resp.Item["sample_frequency"] != nil {
				sensor.SampleFrequency, _ = strconv.ParseInt(*resp.Item["sample_frequency"].N, 10, 64)
			} else {
				sensor.SampleFrequency = 60
			}

			if resp.Item["latitude"] != nil && resp.Item["longitude"] != nil {
				sensor.Latitude, _ = strconv.ParseFloat(*resp.Item["latitude"].N, 64)
				sensor.Longitude, _ = strconv.ParseFloat(*resp.Item["longitude"].N, 64)
			}
		}
	}
	return sensor, err
}

func (d *Database) getTimeOfLastReadingForSensor(sensorId string, accountId string, timestamp *time.Time) (*time.Time, error) {
	sensorReadingsTableName := fmt.Sprintf("sensor_readings_%s", timestamp.Format(TABLE_TIMESTAMP_FORMAT))
	params := &dynamodb.QueryInput{
		TableName: aws.String(sensorReadingsTableName),
		AttributesToGet: []*string{
			aws.String("timestamp"),
		},
		ScanIndexForward: aws.Bool(false),
		KeyConditions: map[string]*dynamodb.Condition{
			"id": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(fmt.Sprintf("%s:%s", accountId, sensorId)),
					},
				},
			},
		},
		Limit: aws.Int64(1),
	}

	var resp *dynamodb.QueryOutput
	var lastReadingTimestamp time.Time
	var err error
	if resp, err = d.dynamoDBService.Query(params); err == nil {
		for _, sensorRecord := range resp.Items {
			var timestampInt64 int64
			if timestampInt64, err = strconv.ParseInt(*sensorRecord["timestamp"].N, 10, 64); err == nil {
				lastReadingTimestamp = time.Unix(timestampInt64, 0)
				return &lastReadingTimestamp, err
			}
		}
	}
	return nil, err
}

func (d *Database) shouldEvaluateSensorReading(readingTimestamp *time.Time, sensor *Sensor) bool {
	var lastReadingTimestamp *time.Time
	var err error
	if lastReadingTimestamp, err = d.getTimeOfLastReadingForSensor(sensor.ID, sensor.AccountID, readingTimestamp); err != nil {
		log.Printf("Error while looking up timestamp of last reading for sensor, proceeding anyway: Sensor ID=%s, Error=%s", sensor.ID, err.Error())
		return true
	}

	if lastReadingTimestamp != nil {
		secondsElapsed := readingTimestamp.Sub(*lastReadingTimestamp).Seconds()
		sampleFrequency := sensor.SampleFrequency
		log.Printf("Seconds since last reading was written: %d", int32(secondsElapsed))
		if secondsElapsed < float64(sampleFrequency-SAMPLE_FREQUENCY_TOLERANCE) {
			log.Printf("Ignoring reading for sensor %s due to sample frequency limit (%d seconds)", sensor.ID, sampleFrequency)
			return false
		}
	}
	return true
}

func (d *Database) createSensor(sensorId string, accountId string) (*Sensor, error) {
	var err error
	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"id": &dynamodb.AttributeValue{
				S: aws.String(fmt.Sprintf("%s", sensorId)),
			},
			"account_id": &dynamodb.AttributeValue{
				S: aws.String(accountId),
			},
			"name": &dynamodb.AttributeValue{
				S: aws.String(" "),
			},
			"state": &dynamodb.AttributeValue{
				S: aws.String("active"),
			},
			"sample_frequency": &dynamodb.AttributeValue{
				N: aws.String("1"),
			},
			"location_enabled": &dynamodb.AttributeValue{
				BOOL: aws.Bool(false),
			},
		},
		TableName: aws.String("sensors"),
	}

	if _, err = d.dynamoDBService.PutItem(input); err != nil {
		log.Printf("Encountered error adding new sensor: %s\n", err.Error())
	}

	sensor := &Sensor{
		ID:              sensorId,
		AccountID:       accountId,
		Name:            " ",
		State:           "active",
		SampleFrequency: 60,
	}
	return sensor, err
}

type Sensor struct {
	ID              string  `json:"id"`
	AccountID       string  `json:"account_id"`
	Name            string  `json:"name"`
	State           string  `json:"state"`
	SampleFrequency int64   `json:"sample_frequency"`
	LocationEnabled bool    `json:"location_enabled"`
	Latitude        float64 `json:"latitude,omitempty"`
	Longitude       float64 `json:"longitude,omitempty"`
}

type Account struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type Measurement struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type MinMaxMeasurement struct {
	Name string      `json:"name"`
	Min  Measurement `json:"min"`
	Max  Measurement `json:"max"`
}

type SensorReadingQueueMessage struct {
	RelayID            string        `json:"relay_id"`
	SensorID           string        `json:"sensor_id"`
	ReadingTimestamp   int32         `json:"reading_timestamp"`
	ReportingTimestamp int32         `json:"reporting_timestamp"`
	Measurements       []Measurement `json:"measurements"`
}
