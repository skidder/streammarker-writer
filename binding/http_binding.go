package binding

import (
	"encoding/json"
	"net/http"

	kitlog "github.com/go-kit/kit/log"
	levlog "github.com/go-kit/kit/log/levels"
	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/skidder/streammarker-writer/config"
	"github.com/skidder/streammarker-writer/endpoint"

	"golang.org/x/net/context"
)

const (
	getHTTPMethod = "GET"
)

// StartHealthCheckHTTPListener creates a Go-routine that has an HTTP listener for the healthcheck endpoint
func StartHealthCheckHTTPListener(logger kitlog.Logger, root context.Context, errc chan error, c *config.Configuration) {
	go func() {
		ctx, cancel := context.WithCancel(root)
		defer cancel()

		l := levlog.New(logger)
		l.Info().Log("HealthCheckAddress", c.HealthCheckAddress, "transport", "HTTP/JSON")

		router := createHealthCheckRouter(logger, ctx, endpoint.NewHealthCheck(c))
		errc <- http.ListenAndServe(c.HealthCheckAddress, router)
	}()
}

func createHealthCheckRouter(logger kitlog.Logger, ctx context.Context, healthCheckEndpoint endpoint.HealthCheckServicer) *mux.Router {
	router := mux.NewRouter()
	router.Handle("/healthcheck",
		kithttp.NewServer(
			ctx,
			healthCheckEndpoint.Run,
			func(*http.Request) (interface{}, error) { return struct{}{}, nil },
			encodeHealthCheckHTTPResponse,
			kithttp.ServerErrorLogger(logger),
		)).Methods(getHTTPMethod)
	return router
}

func encodeHealthCheckHTTPResponse(w http.ResponseWriter, i interface{}) error {
	w.Header().Add("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(i.(*endpoint.HealthCheckResponse))
}
