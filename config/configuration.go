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
	"github.com/urlgrey/streammarker-writer/db"
)

const (
	defaultQueueName = "streammarker-collector-messages"
)

// Configuration holds application configuration details
type Configuration struct {
	HealthCheckAddress string
	SQSService         sqsiface.SQSAPI
	DynamoDBService    dynamodbiface.DynamoDBAPI
	QueueName          string
	QueueURL           string
	Database           *db.Database
}

// LoadConfiguration loads the app config
func LoadConfiguration() *Configuration {
	queueName := os.Getenv("STREAMMARKER_QUEUE_NAME")
	if queueName == "" {
		queueName = defaultQueueName
	}

	// Create external service connections
	s := session.New()
	sqsService := createSQSConnection(s)
	dynamoDBService := createDynamoDBConnection(s)
	queueURL := findQueueURL(sqsService, queueName)
	db := db.NewDatabase(dynamoDBService)

	return &Configuration{
		QueueName:          queueName,
		QueueURL:           queueURL,
		SQSService:         sqsService,
		DynamoDBService:    dynamoDBService,
		HealthCheckAddress: ":3100",
		Database:           db,
	}
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
