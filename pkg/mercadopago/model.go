package mercadopago

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/newdesksoftwares/private-kit/pkg/pb/protocols/webhooks"
)

const Provider = "mercadopago"

const (
	TopicPayment                       = "payment"
	TopicSubscriptionPreapproval       = "subscription_preapproval"
	TopicSubscriptionAuthorizedPayment = "subscription_authorized_payment"
	TopicChargeback                    = "topic_claims_integration_wh"
	TopicComplaint                     = "topic_complaint_integration_wh"
)

const (
	ActionPaymentCreated = "payment.created"
	ActionPaymentUpdated = "payment.updated"
	ActionCreated        = "created"
	ActionUpdated        = "updated"
)

type WebhookMessage struct {
	Signature      string          `json:"signature"`
	RequestId      string          `json:"request_id"`
	NotificationId FlexString      `json:"id"`
	Type           string          `json:"type"`
	Action         string          `json:"action"`
	LiveMode       bool            `json:"live_mode"`
	DataId         string          `json:"data_id"`
	Data           json.RawMessage `json:"data"`
}

func (m *WebhookMessage) BodyDataId() string {
	var d struct {
		ID FlexString `json:"id"`
	}

	if err := json.Unmarshal(m.Data, &d); err != nil {
		return ""
	}

	return string(d.ID)
}

type Event webhooks.WebhooksComplete

func (m *WebhookMessage) Event(status, reference string, resource interface{}) *Event {
	raw, _ := json.Marshal(resource)

	return &Event{
		Id:         m.RequestId,
		Provider:   Provider,
		Type:       m.Type,
		Action:     m.Action,
		ResourceId: m.DataId,
		Reference:  reference,
		Status:     status,
		ReceivedAt: time.Now().Format(time.RFC3339),
		Resource:   string(raw),
	}
}

func (e *Event) Key() string {
	if e.Reference != "" {
		return e.Reference
	}

	return e.Provider + "_" + e.ResourceId
}

func (e *Event) Marshal() []byte {
	b, _ := json.Marshal(e)
	return b
}

type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	*f = FlexString(strings.Trim(string(b), `"`))
	return nil
}

type Payment struct {
	ID                int64      `json:"id"`
	Status            string     `json:"status"`
	StatusDetail      string     `json:"status_detail"`
	ExternalReference string     `json:"external_reference"`
	Description       string     `json:"description"`
	TransactionAmount float64    `json:"transaction_amount"`
	CurrencyID        string     `json:"currency_id"`
	PaymentMethodID   string     `json:"payment_method_id"`
	PaymentTypeID     string     `json:"payment_type_id"`
	Installments      int        `json:"installments"`
	LiveMode          bool       `json:"live_mode"`
	DateCreated       time.Time  `json:"date_created"`
	DateApproved      *time.Time `json:"date_approved"`
	DateLastUpdated   time.Time  `json:"date_last_updated"`

	Payer struct {
		ID    FlexString `json:"id"`
		Email string     `json:"email"`
	} `json:"payer"`

	Metadata json.RawMessage `json:"metadata"`
}

type Preapproval struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"`
	PreapprovalPlanID string     `json:"preapproval_plan_id"`
	ExternalReference string     `json:"external_reference"`
	Reason            string     `json:"reason"`
	PayerID           int64      `json:"payer_id"`
	PayerEmail        string     `json:"payer_email"`
	DateCreated       time.Time  `json:"date_created"`
	LastModified      time.Time  `json:"last_modified"`
	NextPaymentDate   *time.Time `json:"next_payment_date"`

	AutoRecurring struct {
		Frequency         int     `json:"frequency"`
		FrequencyType     string  `json:"frequency_type"`
		TransactionAmount float64 `json:"transaction_amount"`
		CurrencyID        string  `json:"currency_id"`
	} `json:"auto_recurring"`
}

type AuthorizedPayment struct {
	ID                FlexString `json:"id"`
	PreapprovalID     string     `json:"preapproval_id"`
	Status            string     `json:"status"`
	Type              string     `json:"type"`
	ExternalReference string     `json:"external_reference"`
	TransactionAmount float64    `json:"transaction_amount"`
	CurrencyID        string     `json:"currency_id"`
	RetryAttempt      int        `json:"retry_attempt"`
	DateCreated       time.Time  `json:"date_created"`
	NextRetryDate     *time.Time `json:"next_retry_date"`

	Payment struct {
		ID           int64  `json:"id"`
		Status       string `json:"status"`
		StatusDetail string `json:"status_detail"`
	} `json:"payment"`
}

type APIError struct {
	Message    string          `json:"message"`
	Code       string          `json:"error"`
	Status     int             `json:"status"`
	Cause      json.RawMessage `json:"cause"`
	StatusCode int             `json:"-"`
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return "mercadopago: request failed with status " + strconv.Itoa(e.StatusCode)
	}

	return "mercadopago: " + e.Message + " (" + strconv.Itoa(e.StatusCode) + ")"
}
