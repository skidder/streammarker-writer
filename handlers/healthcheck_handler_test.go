package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/codegangsta/negroni"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheckAWSError(t *testing.T) {
	config := &aws.Config{}
	s := session.New()
	h := NewHealthCheckHandler(dynamodb.New(s), sqs.New(s, config), "bogus queue")

	r, _ := http.NewRequest("GET", "/healthcheck", strings.NewReader(""))
	r.Header.Add("Accept", "application/json")
	rec := httptest.NewRecorder()
	rw := negroni.NewResponseWriter(rec)
	rw.Header().Set("Content-Type", "application/json")

	h.HealthCheck(rw, r)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
