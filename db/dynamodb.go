package db

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
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/urlgrey/streammarker-writer/msg"
)

const (
	tableTimestampFormat       = "2006-01"
	hourlyTableTimestampFormat = "2006-01"
	sampleFrequencyTolerance   = 3
)

// DynamoDAO represents a database to be used for reading & writing measurements and
// managing devices
type DynamoDAO struct {
	dynamoDBService dynamodbiface.DynamoDBAPI
}

// NewDynamoDAO builds a new DynamoDAO instance
func NewDynamoDAO(dynamoDBService dynamodbiface.DynamoDBAPI) *DynamoDAO {
	return &DynamoDAO{dynamoDBService: dynamoDBService}
}

// WriteSensorReading will record the Sensor Reading data, first verifying that a corresponding reporting
// device and account exist and are active
func (d *DynamoDAO) WriteSensorReading(r *msg.SensorReadingQueueMessage) error {
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
	if sensor, err = d.GetSensor(r.SensorID, relay.AccountID); err != nil {
		return err
	}

	// if the sensor doesn't exist, then create it and associate with the relay account
	if sensor == nil {
		log.Printf("Sensor not found, adding: %s", r.SensorID)
		if sensor, err = d.CreateSensor(r.SensorID, relay.AccountID); err != nil {
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
	return d.recordMeasurement(r, sensor, &readingTimestamp)
}

func (d *DynamoDAO) recordMeasurement(r *msg.SensorReadingQueueMessage, sensor *Sensor, readingTimestamp *time.Time) error {
	var err error
	var measurementsJSON []byte
	if measurementsJSON, err = json.Marshal(r.Measurements); err != nil {
		return err
	}
	sensorReadingsTableName := fmt.Sprintf("sensor_readings_%s", readingTimestamp.Format(tableTimestampFormat))
	item := map[string]*dynamodb.AttributeValue{
		"id": {
			S: aws.String(fmt.Sprintf("%s:%s", sensor.AccountID, sensor.ID)),
		},
		"timestamp": {
			N: aws.String(fmt.Sprintf("%d", r.ReadingTimestamp)),
		},
		"account_id": {
			S: aws.String(sensor.AccountID),
		},
		"relay_id": {
			S: aws.String(r.RelayID),
		},
		"sensor_id": {
			S: aws.String(sensor.ID),
		},
		"measurements": {
			S: aws.String(string(measurementsJSON)),
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
func (d *DynamoDAO) getTableWaitTime() (t time.Duration) {
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
func (d *DynamoDAO) createSensorReadingsTable(tableName string) (err error) {
	createTableInput := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{ // Required
			{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("timestamp"),
				AttributeType: aws.String("N"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("HASH"),
			},
			{
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

// GetRelay retrieves a relay record by ID
func (d *DynamoDAO) GetRelay(relayID string) (relay *Relay, err error) {
	params := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
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
			err = fmt.Errorf("Relay not found: %s", relayID)
		}
	}
	return
}

// GetSensor retrieves a sensor by ID within the context of an account
func (d *DynamoDAO) GetSensor(sensorID string, accountID string) (*Sensor, error) {
	params := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(sensorID),
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
				ID:              sensorID,
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

func (d *DynamoDAO) getTimeOfLastReadingForSensor(sensorID string, accountID string, timestamp *time.Time) (*time.Time, error) {
	sensorReadingsTableName := fmt.Sprintf("sensor_readings_%s", timestamp.Format(tableTimestampFormat))
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
						S: aws.String(fmt.Sprintf("%s:%s", accountID, sensorID)),
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

func (d *DynamoDAO) shouldEvaluateSensorReading(readingTimestamp *time.Time, sensor *Sensor) bool {
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
		if secondsElapsed < float64(sampleFrequency-sampleFrequencyTolerance) {
			log.Printf("Ignoring reading for sensor %s due to sample frequency limit (%d seconds)", sensor.ID, sampleFrequency)
			return false
		}
	}
	return true
}

// CreateSensor creates a new sensor by ID within an account
func (d *DynamoDAO) CreateSensor(sensorID string, accountID string) (*Sensor, error) {
	var err error
	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(fmt.Sprintf("%s", sensorID)),
			},
			"account_id": {
				S: aws.String(accountID),
			},
			"name": {
				S: aws.String(" "),
			},
			"state": {
				S: aws.String("active"),
			},
			"sample_frequency": {
				N: aws.String("1"),
			},
			"location_enabled": {
				BOOL: aws.Bool(false),
			},
		},
		TableName: aws.String("sensors"),
	}

	if _, err = d.dynamoDBService.PutItem(input); err != nil {
		log.Printf("Encountered error adding new sensor: %s\n", err.Error())
	}

	sensor := &Sensor{
		ID:              sensorID,
		AccountID:       accountID,
		Name:            " ",
		State:           "active",
		SampleFrequency: 60,
	}
	return sensor, err
}

// Sensor represents a Sensor capable of taking measurements
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

// Account reprensets a user account
type Account struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// Relay represents a StreamMarker relay
type Relay struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	State     string `json:"state"`
}

func (r *Relay) isActive() bool {
	return (r.State == "active")
}
