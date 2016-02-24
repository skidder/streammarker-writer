package queue

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/urlgrey/streammarker-writer/dao"
)

// Consumer represents a StreamMarker queue consumer
type Consumer struct {
	db         *dao.Database
	sqsService *sqs.SQS
	queueURL   string

	stopChannel chan bool
	waitGroup   *sync.WaitGroup
}

// NewConsumer constructs a new queue consumer
func NewConsumer(db *dao.Database, sqsService *sqs.SQS, queueURL string) *Consumer {
	return &Consumer{
		db:          db,
		sqsService:  sqsService,
		queueURL:    queueURL,
		stopChannel: make(chan bool),
		waitGroup:   &sync.WaitGroup{},
	}
}

// Run the queue consumer, will block until the Stop function is called
func (q *Consumer) Run() {
	q.waitGroup.Add(1)
	defer q.waitGroup.Done()

	for {
		select {
		case <-q.stopChannel:
			log.Println("Stopping the consumer process")
			return
		default:
		}

		params := &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(1),
			WaitTimeSeconds:     aws.Int64(1),
		}
		resp, err := q.sqsService.ReceiveMessage(params)

		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				// Generic AWS Error with Code, Message, and original error (if any)
				log.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
				if reqErr, ok := err.(awserr.RequestFailure); ok {
					// A service error occurred
					log.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
				}
			} else {
				// This case should never be hit, The SDK should alwsy return an
				// error which satisfies the awserr.Error interface.
				log.Println(err.Error())
			}
		} else {
			for _, msg := range resp.Messages {
				// parse message from JSON
				message := new(dao.SensorReadingQueueMessage)
				json.Unmarshal([]byte(*msg.Body), message)

				// process the message
				if err = q.db.WriteSensorReading(message); err != nil {
					log.Println("Error writing sensor reading to DB: ", err.Error())
				}

				// delete it from the queue
				deleteMessageParams := &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(q.queueURL),
					ReceiptHandle: aws.String(*msg.ReceiptHandle),
				}
				q.sqsService.DeleteMessage(deleteMessageParams)
			}
		}
	}
}

// Stop the queue consumer
func (q *Consumer) Stop() {
	close(q.stopChannel) // signal to the Run goroutine to stop
	q.waitGroup.Wait()   // wait for the Run routine and its children to finish
}
