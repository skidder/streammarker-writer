package queue

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/urlgrey/streammarker-writer/dao"
)

func TestQueueConsumerRunStop(t *testing.T) {
	s := session.New()
	config := &aws.Config{}
	db := dao.NewDatabase(dynamodb.New(s))
	qc := NewQueueConsumer(db, sqs.New(s, config), "asdf")
	go qc.Run()
	time.Sleep(time.Second)
	qc.Stop()

	t.Log("Waiting for queue consumer to return")
	qc.waitGroup.Wait()
}
