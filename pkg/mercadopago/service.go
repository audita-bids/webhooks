package mercadopago

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/log"
	"resty.dev/v3"
)

var ErrMissingDataId = errors.New("mercadopago: notification without data.id")

const (
	MpBaseUrl = "https://api.mercadopago.com"

	pathPayment           = "/v1/payments/%s"
	pathPreapproval       = "/preapproval/%s"
	pathAuthorizedPayment = "/authorized_payments/%s"
	pathChargeback        = "/v1/chargebacks/%s"
	pathMerchantOrder     = "/merchant_orders/%s"
)

type Service interface {
	HandleWebhook(ctx context.Context, webhook *WebhookMessage) (*Event, error)

	GetPayment(ctx context.Context, id string) (*Payment, error)
	GetPreapproval(ctx context.Context, id string) (*Preapproval, error)
	GetAuthorizedPayment(ctx context.Context, id string) (*AuthorizedPayment, error)
	GetChargeback(ctx context.Context, id string) (json.RawMessage, error)
	GetMerchantOrder(ctx context.Context, id string) (json.RawMessage, error)

	Close() error
}

type service struct {
	logger log.Logger
	mpApi  *resty.Client
}

func NewService(logger log.Logger) Service {
	baseUrl := os.Getenv("MP_API_BASE")
	if baseUrl == "" {
		baseUrl = MpBaseUrl
	}

	c := resty.New().
		SetBaseURL(baseUrl).
		SetAuthToken(os.Getenv("MP_ACCESS_TOKEN")).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("User-Agent", "audita-webhooks/1.0").
		SetTimeout(15 * time.Second).
		SetRetryCount(4).
		SetRetryWaitTime(500 * time.Millisecond).
		SetRetryMaxWaitTime(8 * time.Second).
		SetRetryDefaultConditions(false).
		AddRetryConditions(retryOnTransient)

	return &service{
		logger: logger,
		mpApi:  c,
	}
}

func (s *service) Close() error {
	return s.mpApi.Close()
}

func (s *service) HandleWebhook(ctx context.Context, webhook *WebhookMessage) (*Event, error) {
	level.Info(s.logger).Log(
		"provider", Provider,
		"during", "handle_webhook",
		"type", webhook.Type,
		"action", webhook.Action,
		"data_id", webhook.DataId,
	)

	if webhook.DataId == "" {
		return nil, ErrMissingDataId
	}

	switch webhook.Type {
	case TopicPayment:
		p, err := s.GetPayment(ctx, webhook.DataId)
		if err != nil {
			return nil, err
		}

		return webhook.Event(p.Status, p.ExternalReference, p), nil

	case TopicSubscriptionPreapproval:
		p, err := s.GetPreapproval(ctx, webhook.DataId)
		if err != nil {
			return nil, err
		}

		return webhook.Event(p.Status, p.ExternalReference, p), nil

	case TopicSubscriptionAuthorizedPayment:
		p, err := s.GetAuthorizedPayment(ctx, webhook.DataId)
		if err != nil {
			return nil, err
		}

		return webhook.Event(p.Status, p.ExternalReference, p), nil

	case TopicChargeback:
		c, err := s.GetChargeback(ctx, webhook.DataId)
		if err != nil {
			return nil, err
		}

		return webhook.Event("", "", c), nil

	default:
		level.Info(s.logger).Log(
			"provider", Provider,
			"during", "handle_webhook",
			"msg", "unhandled topic",
			"topic", webhook.Type,
		)

		return nil, nil
	}
}

func (s *service) GetPayment(ctx context.Context, id string) (*Payment, error) {
	return get[Payment](ctx, s, fmt.Sprintf(pathPayment, id))
}

func (s *service) GetPreapproval(ctx context.Context, id string) (*Preapproval, error) {
	return get[Preapproval](ctx, s, fmt.Sprintf(pathPreapproval, id))
}

func (s *service) GetAuthorizedPayment(ctx context.Context, id string) (*AuthorizedPayment, error) {
	return get[AuthorizedPayment](ctx, s, fmt.Sprintf(pathAuthorizedPayment, id))
}

func (s *service) GetChargeback(ctx context.Context, id string) (json.RawMessage, error) {
	return getRaw(ctx, s, fmt.Sprintf(pathChargeback, id))
}

func (s *service) GetMerchantOrder(ctx context.Context, id string) (json.RawMessage, error) {
	return getRaw(ctx, s, fmt.Sprintf(pathMerchantOrder, id))
}

func get[T any](ctx context.Context, s *service, path string) (*T, error) {
	var (
		out    T
		apiErr APIError
	)

	res, err := s.mpApi.R().
		SetContext(ctx).
		SetResult(&out).
		SetResultError(&apiErr).
		Get(path)

	if err != nil {
		level.Error(s.logger).Log("provider", "mercadopago", "path", path, "error", err)
		return nil, err
	}

	if res.IsStatusFailure() {
		apiErr.StatusCode = res.StatusCode()
		level.Error(s.logger).Log("provider", "mercadopago", "path", path, "error", apiErr.Error())

		return nil, &apiErr
	}

	return &out, nil
}

func getRaw(ctx context.Context, s *service, path string) (json.RawMessage, error) {
	var apiErr APIError

	res, err := s.mpApi.R().
		SetContext(ctx).
		SetResultError(&apiErr).
		Get(path)

	if err != nil {
		level.Error(s.logger).Log("provider", "mercadopago", "path", path, "error", err)
		return nil, err
	}

	if res.IsStatusFailure() {
		apiErr.StatusCode = res.StatusCode()
		level.Error(s.logger).Log("provider", "mercadopago", "path", path, "error", apiErr.Error())

		return nil, &apiErr
	}

	return json.RawMessage(res.Bytes()), nil
}

func retryOnTransient(res *resty.Response, _ error) bool {
	if res == nil || res.StatusCode() == 0 {
		return true
	}

	switch {
	case res.StatusCode() == http.StatusTooManyRequests:
		return true
	case res.StatusCode() == http.StatusRequestTimeout:
		return true
	case res.StatusCode() >= http.StatusInternalServerError:
		return true
	}

	return false
}
