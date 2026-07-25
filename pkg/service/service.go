package service

import (
	"context"
	"os"
	"webhooks/pkg/mercadopago"

	"github.com/go-kit/log"
	"github.com/newdesksoftwares/private-kit/pkg/pb/protocols/client"
	"github.com/redis/go-redis/v9"
)

type Service interface {
	HandleMercadoPagoWebhook(ctx context.Context, webhook *mercadopago.WebhookMessage) (*mercadopago.Event, error)
}

var (
	MercadoPagoWebhookSecret = os.Getenv("MP_SIGNATURE")
)

const (
	TopicPaymentCreated             = "PAYMENT_CREATED"
	TopicPaymentUpdated             = "PAYMENT_UPDATED"
	TopicSubscriptionUpdated        = "SUBSCRIPTION_UPDATED"
	TopicSubscriptionPaymentUpdated = "SUBSCRIPTION_PAYMENT_UPDATED"
	TopicChargebackCreated          = "CHARGEBACK_CREATED"
)

type service struct {
	logger  log.Logger
	mp      mercadopago.Service
	clients client.ClientServiceClient
}

func NewService(logger log.Logger, redis *redis.Client) Service {
	var svc Service

	{
		svc = &service{
			logger: logger,
			mp:     mercadopago.NewService(logger),
		}
		svc = LoggingMiddleware(logger)(svc)
		svc = RecoveryMiddleware(logger)(svc)
		svc = EventMiddleware(logger)(svc)
		svc = CacheMiddleware(logger, redis)(svc)
		svc = SecurityMiddleware(logger)(svc)
	}

	return svc
}

func (s *service) HandleMercadoPagoWebhook(ctx context.Context, m *mercadopago.WebhookMessage) (*mercadopago.Event, error) {
	return s.mp.HandleWebhook(ctx, m)
}
