package config

import (
	stdlog "log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/skidder/streammarker-writer/db"
)

const (
	defaultQueueName        = "streammarker-collector-messages"
	defaultInfluxDBUsername = "streammarker"
	defaultInfluxDBAddress  = "http://127.0.0.1:8086"
	defaultInfluxDBName     = "streammarker_measurements"
)

// Configuration holds application configuration details
type Configuration struct {
	HealthCheckAddress string
	SQSService         sqsiface.SQSAPI
	DynamoDBService    dynamodbiface.DynamoDBAPI
	QueueName          string
	QueueURL           string
	MeasurementWriter  db.MeasurementWriter
	DeviceManager      db.DeviceManager
}

// LoadConfiguration loads the app config
func LoadConfiguration() (*Configuration, error) {
	queueName := os.Getenv("STREAMMARKER_QUEUE_NAME")
	if queueName == "" {
		queueName = defaultQueueName
	}

	influxDBUsername := os.Getenv("STREAMMARKER_INFLUXDB_USERNAME")
	if influxDBUsername == "" {
		influxDBUsername = defaultInfluxDBUsername
	}
	influxDBPassword := os.Getenv("STREAMMARKER_INFLUXDB_PASSWORD")
	influxDBAddress := os.Getenv("STREAMMARKER_INFLUXDB_ADDRESS")
	if influxDBAddress == "" {
		influxDBAddress = defaultInfluxDBAddress
	}

	influxDBName := os.Getenv("STREAMMARKER_INFLUXDB_NAME")
	if influxDBName == "" {
		influxDBName = defaultInfluxDBName
	}

	// Create external service connections
	s := session.New()
	sqsService := createSQSConnection(s)
	dynamoDBService := createDynamoDBConnection(s)
	queueURL := findQueueURL(sqsService, queueName)
	deviceManager := db.NewDynamoDAO(dynamoDBService)
	measurementWriter, err := db.NewInfluxDAO(influxDBAddress, influxDBUsername, influxDBPassword, influxDBName, deviceManager)

	return &Configuration{
		QueueName:          queueName,
		QueueURL:           queueURL,
		SQSService:         sqsService,
		DynamoDBService:    dynamoDBService,
		HealthCheckAddress: ":3100",
		MeasurementWriter:  measurementWriter,
		DeviceManager:      deviceManager,
	}, err
}

func createSQSConnection(s *session.Session) *sqs.SQS {
	config := &aws.Config{}
	if endpoint := os.Getenv("STREAMMARKER_SQS_ENDPOINT"); endpoint != "" {
		config.Endpoint = &endpoint
	}

	return sqs.New(s, config)
}

func findQueueURL(sqsService *sqs.SQS, queueName string) string {
	// check the environment variable first
	var queueURL string
	if queueURL = os.Getenv("STREAMMARKER_SQS_QUEUE_URL"); queueURL != "" {
		return queueURL
	}

	// otherwise, query SQS for the queue URL
	params := &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	}
	if resp, err := sqsService.GetQueueUrl(params); err == nil {
		queueURL = *resp.QueueUrl
	} else {
		stdlog.Panicf("Unable to retrieve queue URL: %s", err.Error())
	}
	return queueURL
}

func createDynamoDBConnection(s *session.Session) *dynamodb.DynamoDB {
	config := &aws.Config{}
	if endpoint := os.Getenv("STREAMMARKER_DYNAMO_ENDPOINT"); endpoint != "" {
		config.Endpoint = &endpoint
	}

	return dynamodb.New(s, config)
}
