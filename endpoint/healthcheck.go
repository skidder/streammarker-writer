package endpoint

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/skidder/streammarker-writer/config"

	"golang.org/x/net/context"
)

// HealthCheckServicer provides functions for performing component healthcheck
type HealthCheckServicer interface {
	Run(context.Context, interface{}) (interface{}, error)
}

// HealthCheckResponse has fields with healthcheck status
type HealthCheckResponse struct {
	Status string `json:"status"`
}

type healthCheck struct {
	sqsService sqsiface.SQSAPI
	queueName  string
}

// NewHealthCheck creates a new healthcheck
func NewHealthCheck(c *config.Configuration) HealthCheckServicer {
	return &healthCheck{c.SQSService, c.QueueURL}
}

func (h *healthCheck) Run(ctx context.Context, request interface{}) (response interface{}, err error) {
	params := &sqs.ListQueuesInput{
		QueueNamePrefix: aws.String(h.queueName),
	}
	_, err = h.sqsService.ListQueues(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				return nil, fmt.Errorf("Request error code=%s, Message=%s, Original error=%s", reqErr.Code(), reqErr.Message(), reqErr.OrigErr())
			}

			// Generic AWS Error with Code, Message, and original error (if any)
			return nil, fmt.Errorf("AWS error code=%s, Message=%s, Original error=%s", awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
		}

		// This case should never be hit, The SDK should alwsy return an
		// error which satisfies the awserr.Error interface.
		return nil, fmt.Errorf("Generic error: %s", err.Error())
	}

	return &HealthCheckResponse{"OK"}, nil
}
