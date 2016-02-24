package main

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/urlgrey/streammarker-writer/dao"
	"github.com/urlgrey/streammarker-writer/handlers"
	"github.com/urlgrey/streammarker-writer/queue"
)

const (
	defaultQueueName = "streammarker-collector-messages"
)

func main() {
	// Create external service connections
	s := session.New()
	sqsService := createSQSConnection(s)
	dynamoDBService := createDynamoDBConnection(s)

	// get queue name
	queueName := os.Getenv("STREAMMARKER_QUEUE_NAME")
	if queueName == "" {
		queueName = defaultQueueName
	}

	db := dao.NewDatabase(dynamoDBService)
	queueConsumer := queue.NewConsumer(db, sqsService, findQueueURL(sqsService, queueName))
	go queueConsumer.Run()

	// Run healthcheck service
	healthCheckServer := negroni.New()
	healthCheckRouter := mux.NewRouter()
	handlers.InitializeRouterForHealthCheckHandler(healthCheckRouter, dynamoDBService, sqsService, queueName)
	healthCheckServer.UseHandler(healthCheckRouter)
	healthCheckServer.Run(":3100")
}

func createSQSConnection(s *session.Session) *sqs.SQS {
	config := &aws.Config{}
	if endpoint := os.Getenv("STREAMMARKER_SQS_ENDPOINT"); endpoint != "" {
		config.Endpoint = &endpoint
	}

	return sqs.New(s, config)
}

func createDynamoDBConnection(s *session.Session) *dynamodb.DynamoDB {
	config := &aws.Config{}
	if endpoint := os.Getenv("STREAMMARKER_DYNAMO_ENDPOINT"); endpoint != "" {
		config.Endpoint = &endpoint
	}

	return dynamodb.New(s, config)
}

func findQueueURL(sqsService *sqs.SQS, queueName string) (queueURL string) {
	// check the environment variable first
	if queueURL = os.Getenv("STREAMMARKER_SQS_QUEUE_URL"); queueURL != "" {
		return
	}

	// otherwise, query SQS for the queue URL
	params := &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	}
	if resp, err := sqsService.GetQueueUrl(params); err == nil {
		queueURL = *resp.QueueUrl
	} else {
		log.Panicf("Unable to retrieve queue URL: %s", err.Error())
	}
	return
}
