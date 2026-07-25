package service

import (
	"context"
	"fmt"
	"webhooks/pkg/mercadopago"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mercadopago/sdk-go/pkg/webhook"
	"github.com/newdesksoftwares/private-kit/kafka"
	"github.com/redis/go-redis/v9"
	kaf "github.com/segmentio/kafka-go"
)

type Middleware func(Service) Service

func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw *loggingMiddleware) HandleMercadoPagoWebhook(ctx context.Context, request *mercadopago.WebhookMessage) (*mercadopago.Event, error) {
	defer func() {
		mw.logger.Log("method", "HandleMercadoPagoWebhook", "status", "completed")
	}()

	mw.logger.Log("method", "HandleMercadoPagoWebhook", "status", "started")
	return mw.next.HandleMercadoPagoWebhook(ctx, request)
}

func RecoveryMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &recoveryMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type recoveryMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw *recoveryMiddleware) HandleMercadoPagoWebhook(ctx context.Context, request *mercadopago.WebhookMessage) (event *mercadopago.Event, err error) {
	defer func() {
		if r := recover(); r != nil {
			mw.logger.Log("method", "HandleMercadoPagoWebhook", "status", "recovered", "error", r)
			err = fmt.Errorf("recovered from panic: %v", r)
		}
	}()

	return mw.next.HandleMercadoPagoWebhook(ctx, request)
}

func EventMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &eventMiddleware{
			next:     next,
			logger:   logger,
			producer: kafka.NewKafkaProducer(),
		}
	}
}

type eventMiddleware struct {
	next     Service
	producer *kafka.KafkaProducer
	logger   log.Logger
}

func (mw *eventMiddleware) HandleMercadoPagoWebhook(ctx context.Context, request *mercadopago.WebhookMessage) (result *mercadopago.Event, err error) {
	defer func() {
		mw.logger.Log("method", "HandleMercadoPagoWebhook", "sending", "kafka message")

		if err == nil && result != nil {
			var topic string

			switch {
			case result.Action == mercadopago.ActionPaymentCreated:
				topic = TopicPaymentCreated
			case result.Action == mercadopago.ActionPaymentUpdated:
				topic = TopicPaymentUpdated
			case result.Type == mercadopago.TopicSubscriptionPreapproval:
				topic = TopicSubscriptionUpdated
			case result.Type == mercadopago.TopicSubscriptionAuthorizedPayment:
				topic = TopicSubscriptionPaymentUpdated
			case result.Type == mercadopago.TopicChargeback:
				topic = TopicChargebackCreated
			default:
				return
			}

			msg := kaf.Message{
				Topic: topic,
				Key:   []byte(result.Key()),
				Value: result.Marshal(),
			}

			err = mw.producer.Publish(ctx, msg)
			if err != nil {
				mw.logger.Log("method", "HandleMercadoPagoWebhook", "error", err)
			}
		}
	}()

	return mw.next.HandleMercadoPagoWebhook(ctx, request)
}

func CacheMiddleware(logger log.Logger, redis *redis.Client) Middleware {
	return func(next Service) Service {
		return &cacheMiddleware{
			next:   next,
			logger: logger,
			redis:  redis,
		}
	}
}

type cacheMiddleware struct {
	next   Service
	logger log.Logger
	redis  *redis.Client
}

func (mw *cacheMiddleware) HandleMercadoPagoWebhook(ctx context.Context, request *mercadopago.WebhookMessage) (*mercadopago.Event, error) {
	return mw.next.HandleMercadoPagoWebhook(ctx, request)
}

func SecurityMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &securityMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type securityMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw *securityMiddleware) HandleMercadoPagoWebhook(ctx context.Context, request *mercadopago.WebhookMessage) (*mercadopago.Event, error) {
	err := webhook.ValidateSignature(
		request.Signature,
		request.RequestId,
		request.DataId,
		MercadoPagoWebhookSecret,
	)

	if err != nil {
		level.Error(mw.logger).Log("during", "securityMiddleware", "error", err)
		return nil, err
	}

	return mw.next.HandleMercadoPagoWebhook(ctx, request)
}
