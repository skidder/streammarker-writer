package db

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
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
