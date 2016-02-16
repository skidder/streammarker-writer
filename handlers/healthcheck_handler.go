package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gorilla/mux"
)

type HealthCheckHandler struct {
	dynamoDBService *dynamodb.DynamoDB
	sqsService      *sqs.SQS
	queueName       string
}

func NewHealthCheckHandler(dynamoDBService *dynamodb.DynamoDB, sqsService *sqs.SQS, queueName string) *HealthCheckHandler {
	return &HealthCheckHandler{sqsService: sqsService, queueName: queueName}
}

// Add routes to router
func InitializeRouterForHealthCheckHandler(r *mux.Router, dynamoDBService *dynamodb.DynamoDB, sqsService *sqs.SQS, queueName string) {
	m := NewHealthCheckHandler(dynamoDBService, sqsService, queueName)
	r.HandleFunc("/healthcheck", m.HealthCheck).Methods("GET")
}

// Examine and report the health of the component and dependencies
func (h *HealthCheckHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	params := &sqs.ListQueuesInput{
		QueueNamePrefix: aws.String(h.queueName),
	}
	_, err := h.sqsService.ListQueues(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Generic AWS Error with Code, Message, and original error (if any)
			log.Printf("AWS error code=%s, Message=%s, Original error=%s", awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				log.Printf("Request error code=%s, Message=%s, Original error=%s", reqErr.Code(), reqErr.Message(), reqErr.OrigErr())
			}
		} else {
			// This case should never be hit, The SDK should alwsy return an
			// error which satisfies the awserr.Error interface.
			log.Printf("Generic error: %s", err.Error())
		}

		http.Error(w,
			fmt.Sprintf("{\"error\": \"Error checking SQS connectivity: %+v\"}", err),
			http.StatusInternalServerError)
	}
}
