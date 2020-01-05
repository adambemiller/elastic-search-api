package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"
)

type server struct {
	logger        *logrus.Logger
	router        *chi.Mux
	elasticClient *elasticsearch.Client
}

func handler() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	s := server{
		logger: logger,
	}
	s.router = chi.NewRouter()
	s.routes()

	elasticClient, err := CreateElasticClient(cfg)
	if err != nil {
		logger.Fatal(err)
	}

	s.elasticClient = elasticClient

	const port = ":8080"
	server := http.Server{
		Addr:    port,
		Handler: s.router,
	}

	go func(server *http.Server) {
		logger.Info("Server listening on", port)
		if err := server.ListenAndServe(); err != nil {
			s.logger.Error(err.Error())
		}
	}(&server)

	// capture interrupt (ctrl-c)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// wait indefinitely until interrupt signal
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		s.logger.Fatal(err.Error())
	}

}

func main() {
	handler()
}
