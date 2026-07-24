package transports

import (
	"context"
	"encoding/json"
	"net/http"
	"webhooks/pkg/endpoint"
	apperrors "webhooks/pkg/errors"
	model "webhooks/pkg/mercadopago"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"go.elastic.co/apm/module/apmgorilla/v2"
)

func NewHTTPServer(endpoint endpoint.EndpointSetup, logger log.Logger) http.Handler {
	r := mux.NewRouter()
	apmgorilla.Instrument(r)

	r.Methods(http.MethodPost).
		Path("/v1/webhooks/mercado-pago").
		Handler(httptransport.NewServer(
			endpoint.HandleMercadoPagoWebhook,
			decodeMercadoPagoWebhookHTTP,
			encodeHttpResponse,
		))

	return r
}

func decodeMercadoPagoWebhookHTTP(ctx context.Context, r *http.Request) (interface{}, error) {
	var webhook model.WebhookMessage

	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		return nil, err
	}

	qr := r.URL.Query()
	hdr := r.Header

	webhook.DataId = qr.Get("data.id")
	webhook.Signature = hdr.Get("x-signature")
	webhook.RequestId = hdr.Get("x-request-id")

	return &webhook, nil
}

func encodeHttpResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if resp, ok := response.(*endpoint.Resp); ok {
		if resp.Error != nil {
			httpErr := apperrors.ParseError(resp.Error)
			writeError(w, httpErr)
			return nil
		}
	}

	if err, ok := response.(error); ok {
		httpErr := apperrors.ParseError(err)
		writeError(w, httpErr)
		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, httpErr *apperrors.HTTPError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErr.Status)

	if err := json.NewEncoder(w).Encode(httpErr); err != nil {
	}
}
