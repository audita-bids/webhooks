package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"syscall"
	"time"
	"webhooks/options"

	"webhooks/pkg/endpoint"
	"webhooks/pkg/service"
	"webhooks/transports"

	"github.com/audita-bids/private-kit/middlewares"
	"github.com/audita-bids/private-kit/pkg/lib"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
)

func main() {
	instanceCfg := options.NewCfg()
	cfg := options.HandleCfg(instanceCfg)

	logger := lib.SetupLogger(cfg.Debug)

	redis, err := lib.Initiate()

	if err != nil {
		level.Error(logger).Log("msg", "failed to connect to redis", "err", err)
		os.Exit(1)
	}
	var (
		svc       = service.NewService(logger, redis.GetClient())
		endpoints = endpoint.NewEndpointSetup(svc, logger)
		// kafkaHandler = transports.NewKafkaConsumers(*endpoints)
		httpHandler = transports.NewHTTPServer(*endpoints, logger)

		httpServer *http.Server
	)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ready := new(atomic.Bool)

	var g run.Group
	{
		promListener, err := net.Listen("tcp", cfg.PromAddr)
		if err != nil {
			level.Error(logger).Log("msg", "failed to listen on prometheus address", "err", err)
			os.Exit(1)
		}

		config := middlewares.MetricsConfig{
			Logger:         logger,
			EnableEndpoint: true,
			EnableHTTP:     true,
			ServiceName:    "webhooks",
			Ready:          ready,
		}

		srv := middlewares.NewMetricsServer(config, cfg.PromAddr)

		g.Add(func() error {
			level.Info(logger).Log(
				"msg", "prometheus server started",
				"addr", cfg.PromAddr,
			)

			return srv.Serve(promListener)
		}, func(error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if err := srv.Shutdown(ctx); err != nil {
				level.Error(logger).Log("msg", "failed to shutdown prometheus server", "err", err)
			}
		})
	}
	{
		httpListener, err := net.Listen("tcp", cfg.HttpAddr)
		if err != nil {
			level.Error(logger).Log("msg", "failed to listen on http address", "err", err)
			os.Exit(1)
		}

		strip := http.StripPrefix("/api", httpHandler)

		httpServer = &http.Server{
			Handler:           strip,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      120 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		g.Add(func() error {
			return httpServer.Serve(httpListener)
		}, func(error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if err := httpServer.Shutdown(ctx); err != nil {
				level.Error(logger).Log("msg", "failed to shutdown http server", "err", err)
			}
		})
	}

	{
		g.Add(run.SignalHandler(ctx, syscall.SIGINT, syscall.SIGTERM))
	}

	ready.Store(true)

	if err := g.Run(); err != nil {
		level.Error(logger).Log("msg", "servers failed", "err", err)
		os.Exit(1)
	}
}
