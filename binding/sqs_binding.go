package binding

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	kitlog "github.com/go-kit/kit/log"
	levlog "github.com/go-kit/kit/log/levels"
	"github.com/urlgrey/streammarker-writer/config"
	"github.com/urlgrey/streammarker-writer/db"
	"github.com/urlgrey/streammarker-writer/endpoint"

	"golang.org/x/net/context"
)

// StartApplicationSQSConsumer creates a Go-routine that consumes from an SQS queue
func StartApplicationSQSConsumer(logger kitlog.Logger, root context.Context, errc chan error, c *config.Configuration) {
	go func() {
		ctx, cancel := context.WithCancel(root)
		defer cancel()

		l := levlog.New(logger)
		l.Info().Log("ApplicationSQSConsumer queue", c.QueueName, "transport", "SQS")

		errc <- consumeMessagesFromQueue(l, ctx, c)
	}()
}

func consumeMessagesFromQueue(logger levlog.Levels, ctx context.Context, c *config.Configuration) error {
	writerEndpoint := endpoint.NewMessageWriter(c)
	for {
		params := &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.QueueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(1),
			WaitTimeSeconds:     aws.Int64(1),
		}
		resp, err := c.SQSService.ReceiveMessage(params)

		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				// Generic AWS Error with Code, Message, and original error (if any)
				logger.Error().Log("code", awsErr.Code(), "message", awsErr.Message(), "details", awsErr.OrigErr())
				if reqErr, ok := err.(awserr.RequestFailure); ok {
					// A service error occurred
					logger.Error().Log("code", reqErr.Code(), "message", reqErr.Message(), "status_code", reqErr.StatusCode(), "request_id", reqErr.RequestID())
				}
			} else {
				// This case should never be hit, The SDK should always return an
				// error which satisfies the awserr.Error interface.
				logger.Crit().Log("details", err.Error())
				return err
			}
		} else {
			for _, msg := range resp.Messages {
				// parse message from JSON
				message := new(db.SensorReadingQueueMessage)
				json.Unmarshal([]byte(*msg.Body), message)

				// process the message
				_, err = writerEndpoint.Run(ctx, message)
				if err != nil {
					logger.Error().Log("message", "Error processing message from SQS", "details", err.Error())
				}

				// delete it from the queue
				deleteMessageParams := &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(c.QueueURL),
					ReceiptHandle: aws.String(*msg.ReceiptHandle),
				}
				c.SQSService.DeleteMessage(deleteMessageParams)
			}
		}
	}
}
