package endpoint

import (
	"context"
	"webhooks/pkg/mercadopago"
	"webhooks/pkg/service"

	"github.com/audita-bids/private-kit/middlewares"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/log"
)

type EndpointSetup struct {
	HandleMercadoPagoWebhook endpoint.Endpoint
}

func NewEndpointSetup(s service.Service, logger log.Logger) *EndpointSetup {
	var handleMercadoPagoWebhookEndpoint endpoint.Endpoint

	loggingMiddleware := middlewares.EndpointLoggingMiddleware(logger, "webhooks")
	metricsMiddleware := middlewares.MetricsMiddleware("webhooks")
	{
		handleMercadoPagoWebhookEndpoint = MakeHandleMercadoPagoWebhookEndpoint(s)
		handleMercadoPagoWebhookEndpoint = loggingMiddleware("HandleMercadoPagoWebhook")(handleMercadoPagoWebhookEndpoint)
		handleMercadoPagoWebhookEndpoint = metricsMiddleware("HandleMercadoPagoWebhook")(handleMercadoPagoWebhookEndpoint)
	}

	return &EndpointSetup{
		HandleMercadoPagoWebhook: handleMercadoPagoWebhookEndpoint,
	}
}

func MakeHandleMercadoPagoWebhookEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(*mercadopago.WebhookMessage)

		resp, err := s.HandleMercadoPagoWebhook(ctx, req)
		if err != nil {
			return nil, err
		}

		return resp, nil
	}
}

type Resp struct {
	Error  error       `json:"error,omitempty"`
	Items  interface{} `json:"items,omitempty"`
	Total  int64       `json:"total,omitempty"`
	Cursor string      `json:"cursor,omitempty"`
}
