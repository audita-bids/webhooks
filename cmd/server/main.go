package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"webhooks/options"

	"webhooks/pkg/endpoint"
	"webhooks/pkg/service"
	"webhooks/transports"

	"github.com/go-kit/kit/log/level"
	"github.com/newdesksoftwares/private-kit/middlewares"
	"github.com/newdesksoftwares/private-kit/pkg/lib"
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

	var g run.Group
	{
		promListener, err := net.Listen("tcp", cfg.PromAddr)
		config := middlewares.MetricsConfig{
			Logger:         logger,
			EnableEndpoint: true,
			EnableHTTP:     true,
			ServiceName:    "webhooks",
		}

		srv := middlewares.NewMetricsServer(config, cfg.PromAddr)

		g.Add(func() error {
			level.Info(logger).Log(
				"msg", "prometheus server started",
				"addr", cfg.PromAddr,
			)

			return srv.Serve(promListener)
		}, func(error) {
			level.Error(logger).Log(
				"msg", "failed to listen prometheus address",
				"err", err,
			)
		})
	}
	{
		httpListener, err := net.Listen("tcp", cfg.HttpAddr)
		if err != nil {
			level.Error(logger).Log("msg", "failed to listen on http address", "err", err)
			os.Exit(1)
		}

		g.Add(func() error {
			strip := http.StripPrefix("/api", httpHandler)

			level.Info(logger).Log(
				"msg", "http server started",
				"addr", cfg.HttpAddr,
			)

			httpServer = &http.Server{
				Handler: strip,
			}

			return httpServer.Serve(httpListener)
		}, func(error) {
			level.Error(logger).Log("msg", "failed to listen on http address", "err", err)
		})
	}

	if err := g.Run(); err != nil {
		level.Error(logger).Log("msg", "servers failed", "err", err)
		os.Exit(1)
	}
}
