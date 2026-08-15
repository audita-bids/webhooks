# Audita Webhooks

A generic webhook receiver for external integrations.

The goal of this service is to provide a single entry point for messages coming from outside the Audita ecosystem.

External providers should not need to know how Audita's internal services communicate.

They simply send an HTTP webhook.

From there, the webhook service takes care of forwarding the message to the internal processing layer.

## Architecture

The service sits at the boundary between external systems and Audita's internal infrastructure.

```text
                         External Systems
                                │
                                │ HTTP
                                ▼
                    ┌──────────────────────┐
                    │   Audita Webhooks    │
                    │                      │
                    │   Receive & validate │
                    │       messages       │
                    └──────────┬───────────┘
                               │
                    ┌──────────┴───────────┐
                    │                      │
                    │ internal transport   │
                    │                      │
                    ▼                      ▼
               ┌─────────┐           ┌─────────┐
               │  Kafka  │           │  gRPC   │
               └────┬────┘           └────┬────┘
                    │                     │
                    └──────────┬──────────┘
                               ▼
                     Internal processing
```

The important part is the boundary:

```text
External world
      │
      │ HTTP / Webhook
      ▼
┌─────────────────┐
│     Webhooks    │
└────────┬────────┘
         │
         │ Internal protocol
         ▼
  Audita services
```

External integrations only need to understand HTTP.

Everything after that is an internal concern.

## Why a generic webhook service?

Audita integrates with systems that live outside our infrastructure.

These systems may send notifications, events, status updates, documents, or other kinds of messages.

Without a common entry point, every integration could end up implementing its own HTTP endpoint and its own way of forwarding data internally.

This service provides a common boundary instead.

```text
Provider A ──┐
Provider B ──┤
Provider C ──┼──► Webhooks ──► Internal processing
Provider D ──┘
```

This makes integrations easier to add while keeping the internal architecture independent from the external provider.

## Message flow

A typical webhook request follows this path:

```text
1. External system sends HTTP request
                │
                ▼
2. Webhook receives the message
                │
                ▼
3. Request is decoded and validated
                │
                ▼
4. Message is transformed into an
   internal representation
                │
                ▼
5. Message is forwarded internally
                │
                ├────► Kafka
                │
                └────► gRPC
                │
                ▼
6. Internal service processes it
```

The webhook itself should remain intentionally lightweight.

It is not the place where the actual business processing happens.

Its job is to **receive, normalize, and hand off**.

## Kafka or gRPC?

The internal transport does not need to be coupled to the external webhook.

Depending on the use case, a message can be forwarded through different mechanisms.

### Kafka

Kafka is useful when the message should enter an asynchronous processing pipeline.

```text
Webhook
   │
   ▼
Kafka topic
   │
   ├──► Consumer A
   ├──► Consumer B
   └──► Consumer C
```

This allows multiple consumers to react to the same event and decouples the webhook from the processing speed of downstream services.

### gRPC

gRPC is useful when the webhook needs to hand the request directly to a specific internal service.

```text
Webhook
   │
   │ gRPC
   ▼
Internal service
   │
   ▼
Processing
```

The choice belongs to the internal architecture, not to the external integration.

The external provider still sees the same thing:

```text
POST /webhook
```

## Keeping the boundary clean

One of the main design goals is to avoid leaking internal architecture into external integrations.

A provider should not care whether Audita uses:

* Kafka
* gRPC
* Redis
* HTTP
* Another internal service

It only needs to send a valid webhook.

That gives us the freedom to change the internal processing architecture without requiring changes from every external integration.

## Project structure

The repository follows a Go Kit-style structure:

```text
.
├── cmd/
│   └── server/
│       └── main.go
│
├── options/
│   └── ...
│
├── pkg/
│   └── ...
│
├── transports/
│   └── ...
│
├── .air.toml
├── go.mod
└── README.md
```

### `cmd/server`

Application entrypoint and server initialization.

### `options`

Runtime configuration and application options.

### `pkg`

Core application logic and webhook handling.

### `transports`

External transport implementations, keeping HTTP concerns separated from the application logic.

## Go Kit

The service uses Go Kit to keep the transport layer separated from the core application flow.

Conceptually:

```text
HTTP
 │
 ▼
Transport
 │
 ▼
Endpoint
 │
 ▼
Service
 │
 ▼
Internal message
 │
 ▼
Kafka / gRPC
```

This makes the service easier to evolve as new webhook types and internal transports are introduced.

## Example

An external provider can simply send:

```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "event": "some.external.event",
    "data": {
      "id": "123",
      "status": "completed"
    }
  }'
```

The webhook receives the request and takes care of moving it into the internal processing pipeline.

The provider doesn't need to know what happens after that.

## Running locally

### Requirements

* Go
* Access to the internal transport used by the configured integration

Run the server with:

```bash
go run ./cmd/server
```

For development, the repository also contains an Air configuration for live reloading.

## Role in the Audita architecture

The webhook service is the **entry point for external events**.

Together with the API Gateway, it forms two different boundaries around the internal services:

```text
                    External world
                          │
             ┌────────────┴────────────┐
             │                         │
             │ HTTP API                │ Webhooks
             │                         │
             ▼                         ▼
      ┌──────────────┐         ┌──────────────┐
      │ API Gateway  │         │   Webhooks   │
      └──────┬───────┘         └──────┬───────┘
             │                        │
             │ gRPC                   │ Kafka / gRPC
             │                        │
             └───────────┬────────────┘
                         ▼
                 Internal services
```

The responsibilities are intentionally different:

**API Gateway**

> "A client wants to call an Audita API."

**Webhooks**

> "An external system has sent us an event."

**Internal services**

> "Now that we have the message, let's actually process it."

That separation keeps the boundaries between Audita and the outside world simple, while allowing the internal architecture to evolve independently.

---

Built with Go and Go Kit for Audita.
