// Package "main" provides primitives to interact with the AsyncAPI specification.
//
// Code generated by github.com/hound672/asyncapi-codegen version (devel) DO NOT EDIT.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hound672/asyncapi-codegen/pkg/extensions"

	"github.com/google/uuid"
)

// UserController is the structure that provides sending capabilities to the
// developer and and connect the broker with the User
type UserController struct {
	controller
}

// NewUserController links the User to the broker
func NewUserController(bc extensions.BrokerController, options ...ControllerOption) (*UserController, error) {
	// Check if broker controller has been provided
	if bc == nil {
		return nil, extensions.ErrNilBrokerController
	}

	// Create default controller
	controller := controller{
		broker:        bc,
		subscriptions: make(map[string]extensions.BrokerChannelSubscription),
		logger:        extensions.DummyLogger{},
		middlewares:   make([]extensions.Middleware, 0),
		errorHandler:  extensions.DefaultErrorHandler(),
	}

	// Apply options
	for _, option := range options {
		option(&controller)
	}

	return &UserController{controller: controller}, nil
}

func (c UserController) wrapMiddlewares(
	middlewares []extensions.Middleware,
	callback extensions.NextMiddleware,
) func(ctx context.Context, msg *extensions.BrokerMessage) error {
	var called bool

	// If there is no more middleware
	if len(middlewares) == 0 {
		return func(ctx context.Context, msg *extensions.BrokerMessage) error {
			// Call the callback if it exists and it has not been called already
			if callback != nil && !called {
				called = true
				return callback(ctx)
			}

			// Nil can be returned, as the callback has already been called
			return nil
		}
	}

	// Get the next function to call from next middlewares or callback
	next := c.wrapMiddlewares(middlewares[1:], callback)

	// Wrap middleware into a check function that will call execute the middleware
	// and call the next wrapped middleware if the returned function has not been
	// called already
	return func(ctx context.Context, msg *extensions.BrokerMessage) error {
		// Call the middleware and the following if it has not been done already
		if !called {
			// Create the next call with the context and the message
			nextWithArgs := func(ctx context.Context) error {
				return next(ctx, msg)
			}

			// Call the middleware and register it as already called
			called = true
			if err := middlewares[0](ctx, msg, nextWithArgs); err != nil {
				return err
			}

			// If next has already been called in middleware, it should not be executed again
			return nextWithArgs(ctx)
		}

		// Nil can be returned, as the next middleware has already been called
		return nil
	}
}

func (c UserController) executeMiddlewares(ctx context.Context, msg *extensions.BrokerMessage, callback extensions.NextMiddleware) error {
	// Wrap middleware to have 'next' function when calling them
	wrapped := c.wrapMiddlewares(c.middlewares, callback)

	// Execute wrapped middlewares
	return wrapped(ctx, msg)
}

func addUserContextValues(ctx context.Context, addr string) context.Context {
	ctx = context.WithValue(ctx, extensions.ContextKeyIsVersion, "1.0.0")
	ctx = context.WithValue(ctx, extensions.ContextKeyIsProvider, "user")
	return context.WithValue(ctx, extensions.ContextKeyIsChannel, addr)
}

// Close will clean up any existing resources on the controller
func (c *UserController) Close(ctx context.Context) {
	// Unsubscribing remaining channels
}

// SendToPingRequestOperation will send a Ping message on Ping channel.
//
// NOTE: this won't wait for reply, use the normal version to get the reply or do the catching reply manually.
// NOTE: for now, this only support the first message from AsyncAPI list.
// If you need support for other messages, please raise an issue.
func (c *UserController) SendToPingRequestOperation(
	ctx context.Context,
	msg PingMessage,
) error {
	// Set channel address
	addr := "ping.v3"

	// Set correlation ID if it does not exist
	if id := msg.CorrelationID(); id == "" {
		msg.SetCorrelationID(uuid.New().String())
	}

	// Set context
	ctx = addUserContextValues(ctx, addr)
	ctx = context.WithValue(ctx, extensions.ContextKeyIsDirection, "publication")
	ctx = context.WithValue(ctx, extensions.ContextKeyIsCorrelationID, msg.CorrelationID())

	// Convert to BrokerMessage
	brokerMsg, err := msg.toBrokerMessage()
	if err != nil {
		return err
	}

	// Set broker message to context
	ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, brokerMsg.String())

	// Send the message on event-broker through middlewares
	return c.executeMiddlewares(ctx, &brokerMsg, func(ctx context.Context) error {
		return c.broker.Publish(ctx, addr, brokerMsg)
	})
}

// RequestToPingRequestOperation will send a Ping message on Ping channel
// and wait for a Pong message from Pong channel.
//
// If a correlation ID is set in the AsyncAPI, then this will wait for the
// reply with the same correlation ID. Otherwise, it will returns the first
// message on the reply channel.
//
// A timeout can be set in context to avoid blocking operation, if needed.

func (c *UserController) RequestToPingRequestOperation(
	ctx context.Context,
	msg PingMessage,
) (PongMessage, error) {
	// Get receiving channel address
	addr := "pong.v3"

	// Set context
	ctx = addUserContextValues(ctx, addr)

	// Subscribe to broker channel
	sub, err := c.broker.Subscribe(ctx, addr)
	if err != nil {
		c.logger.Error(ctx, err.Error())
		return PongMessage{}, err
	}
	c.logger.Info(ctx, "Subscribed to channel")

	// Close receiver on leave
	defer func() {
		// Stop the subscription
		sub.Cancel(ctx)

		// Logging unsubscribing
		c.logger.Info(ctx, "Unsubscribed from channel")
	}()

	// Set correlation ID if it does not exist
	if id := msg.CorrelationID(); id == "" {
		msg.SetCorrelationID(uuid.New().String())
	}

	// Send the message
	if err := c.SendToPingRequestOperation(ctx, msg); err != nil {
		c.logger.Error(ctx, "error happened when sending message", extensions.LogInfo{Key: "error", Value: err.Error()})
		return PongMessage{}, fmt.Errorf("error happened when sending message: %w", err)
	}

	// Wait for corresponding response
	for {
		select {
		case acknowledgeableBrokerMessage, open := <-sub.MessagesChannel():
			// If subscription is closed and there is no more message
			// (i.e. uninitialized message), then the subscription ended before
			// receiving the expected message
			if !open && acknowledgeableBrokerMessage.IsUninitialized() {
				c.logger.Error(ctx, "Channel closed before getting message")
				return PongMessage{}, extensions.ErrSubscriptionCanceled
			}

			// Get new message
			rmsg, err := newPongMessageFromBrokerMessage(acknowledgeableBrokerMessage.BrokerMessage)
			if err != nil {
				c.logger.Error(ctx, err.Error())
			}

			acknowledgeableBrokerMessage.Ack()

			// If message doesn't have corresponding correlation ID, then ingore and continue
			if msg.CorrelationID() != rmsg.CorrelationID() {
				continue
			}

			// Set context with received values as it is the expected message
			msgCtx := context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, acknowledgeableBrokerMessage.String())
			msgCtx = context.WithValue(msgCtx, extensions.ContextKeyIsDirection, "reception")
			msgCtx = context.WithValue(msgCtx, extensions.ContextKeyIsCorrelationID, msg.CorrelationID())

			// Execute middlewares before returning
			if err := c.executeMiddlewares(msgCtx, &acknowledgeableBrokerMessage.BrokerMessage, nil); err != nil {
				return PongMessage{}, err
			}

			// Return the message to the caller
			//
			// NOTE: it is transformed from the broker again, as it could have
			// been modified by middlewares
			return newPongMessageFromBrokerMessage(acknowledgeableBrokerMessage.BrokerMessage)
		case <-ctx.Done(): // Set corrsponding error if context is done
			c.logger.Error(ctx, "Context done before getting message")
			return PongMessage{}, extensions.ErrContextCanceled
		}
	}
}

// AsyncAPIVersion is the version of the used AsyncAPI document
const AsyncAPIVersion = "1.0.0"

// controller is the controller that will be used to communicate with the broker
// It will be used internally by AppController and UserController
type controller struct {
	// broker is the broker controller that will be used to communicate
	broker extensions.BrokerController
	// subscriptions is a map of all subscriptions
	subscriptions map[string]extensions.BrokerChannelSubscription
	// logger is the logger that will be used² to log operations on controller
	logger extensions.Logger
	// middlewares are the middlewares that will be executed when sending or
	// receiving messages
	middlewares []extensions.Middleware
	// handler to handle errors from consumers and middlewares
	errorHandler extensions.ErrorHandler
}

// ControllerOption is the type of the options that can be passed
// when creating a new Controller
type ControllerOption func(controller *controller)

// WithLogger attaches a logger to the controller
func WithLogger(logger extensions.Logger) ControllerOption {
	return func(controller *controller) {
		controller.logger = logger
	}
}

// WithMiddlewares attaches middlewares that will be executed when sending or receiving messages
func WithMiddlewares(middlewares ...extensions.Middleware) ControllerOption {
	return func(controller *controller) {
		controller.middlewares = middlewares
	}
}

// WithErrorHandler attaches a errorhandler to handle errors from subscriber functions
func WithErrorHandler(handler extensions.ErrorHandler) ControllerOption {
	return func(controller *controller) {
		controller.errorHandler = handler
	}
}

type MessageWithCorrelationID interface {
	CorrelationID() string
	SetCorrelationID(id string)
}

type Error struct {
	Channel string
	Err     error
}

func (e *Error) Error() string {
	return fmt.Sprintf("channel %q: err %v", e.Channel, e.Err)
}

// Message 'PingMessage' reference another one at '#/components/messages/ping'.
// This should be fixed in a future version to allow message override.
// If you encounter this message, feel free to open an issue on this subject
// to let know that you need this functionnality.

// Message 'PongMessage' reference another one at '#/components/messages/pong'.
// This should be fixed in a future version to allow message override.
// If you encounter this message, feel free to open an issue on this subject
// to let know that you need this functionnality.

// PingMessageHeaders is a schema from the AsyncAPI specification required in messages
type PingMessageHeaders struct {
	// Description: Correlation ID set by user
	CorrelationId *string `json:"correlation_id"`
}

// PingMessagePayload is a schema from the AsyncAPI specification required in messages
type PingMessagePayload struct {
	Event *string `json:"event"`
}

// PingMessage is the message expected for 'PingMessage' channel.
type PingMessage struct {
	// Headers will be used to fill the message headers
	Headers PingMessageHeaders

	// Payload will be inserted in the message payload
	Payload PingMessagePayload
}

func NewPingMessage() PingMessage {
	var msg PingMessage

	// Set correlation ID
	u := uuid.New().String()
	msg.Headers.CorrelationId = &u

	return msg
}

// newPingMessageFromBrokerMessage will fill a new PingMessage with data from generic broker message
func newPingMessageFromBrokerMessage(bMsg extensions.BrokerMessage) (PingMessage, error) {
	var msg PingMessage

	// Unmarshal payload to expected message payload format
	err := json.Unmarshal(bMsg.Payload, &msg.Payload)
	if err != nil {
		return msg, err
	}

	// Get each headers from broker message
	for k, v := range bMsg.Headers {
		switch {
		case k == "correlationId": // Retrieving CorrelationId header
			h := string(v)
			msg.Headers.CorrelationId = &h
		default:
			// TODO: log unknown error
		}
	}

	// TODO: run checks on msg type

	return msg, nil
}

// toBrokerMessage will generate a generic broker message from PingMessage data
func (msg PingMessage) toBrokerMessage() (extensions.BrokerMessage, error) {
	// TODO: implement checks on message

	// Marshal payload to JSON
	payload, err := json.Marshal(msg.Payload)
	if err != nil {
		return extensions.BrokerMessage{}, err
	}

	// Add each headers to broker message
	headers := make(map[string][]byte, 1)

	// Adding CorrelationId header
	if msg.Headers.CorrelationId != nil {
		headers["correlationId"] = []byte(*msg.Headers.CorrelationId)
	}

	return extensions.BrokerMessage{
		Headers: headers,
		Payload: payload,
	}, nil
}

// CorrelationID will give the correlation ID of the message, based on AsyncAPI spec
func (msg PingMessage) CorrelationID() string {
	if msg.Headers.CorrelationId != nil {
		return *msg.Headers.CorrelationId
	}

	return ""
}

// SetCorrelationID will set the correlation ID of the message, based on AsyncAPI spec
func (msg *PingMessage) SetCorrelationID(id string) {
	msg.Headers.CorrelationId = &id
}

// SetAsResponseFrom will correlate the message with the one passed in parameter.
// It will assign the 'req' message correlation ID to the message correlation ID,
// both specified in AsyncAPI spec.
func (msg *PingMessage) SetAsResponseFrom(req MessageWithCorrelationID) {
	id := req.CorrelationID()
	msg.Headers.CorrelationId = &id
}

// PongMessageHeaders is a schema from the AsyncAPI specification required in messages
type PongMessageHeaders struct {
	// Description: Correlation ID set by user
	CorrelationId *string `json:"correlation_id"`
}

// PongMessagePayload is a schema from the AsyncAPI specification required in messages
type PongMessagePayload struct {
	Event *string `json:"event"`
}

// PongMessage is the message expected for 'PongMessage' channel.
type PongMessage struct {
	// Headers will be used to fill the message headers
	Headers PongMessageHeaders

	// Payload will be inserted in the message payload
	Payload PongMessagePayload
}

func NewPongMessage() PongMessage {
	var msg PongMessage

	// Set correlation ID
	u := uuid.New().String()
	msg.Headers.CorrelationId = &u

	return msg
}

// newPongMessageFromBrokerMessage will fill a new PongMessage with data from generic broker message
func newPongMessageFromBrokerMessage(bMsg extensions.BrokerMessage) (PongMessage, error) {
	var msg PongMessage

	// Unmarshal payload to expected message payload format
	err := json.Unmarshal(bMsg.Payload, &msg.Payload)
	if err != nil {
		return msg, err
	}

	// Get each headers from broker message
	for k, v := range bMsg.Headers {
		switch {
		case k == "correlationId": // Retrieving CorrelationId header
			h := string(v)
			msg.Headers.CorrelationId = &h
		default:
			// TODO: log unknown error
		}
	}

	// TODO: run checks on msg type

	return msg, nil
}

// toBrokerMessage will generate a generic broker message from PongMessage data
func (msg PongMessage) toBrokerMessage() (extensions.BrokerMessage, error) {
	// TODO: implement checks on message

	// Marshal payload to JSON
	payload, err := json.Marshal(msg.Payload)
	if err != nil {
		return extensions.BrokerMessage{}, err
	}

	// Add each headers to broker message
	headers := make(map[string][]byte, 1)

	// Adding CorrelationId header
	if msg.Headers.CorrelationId != nil {
		headers["correlationId"] = []byte(*msg.Headers.CorrelationId)
	}

	return extensions.BrokerMessage{
		Headers: headers,
		Payload: payload,
	}, nil
}

// CorrelationID will give the correlation ID of the message, based on AsyncAPI spec
func (msg PongMessage) CorrelationID() string {
	if msg.Headers.CorrelationId != nil {
		return *msg.Headers.CorrelationId
	}

	return ""
}

// SetCorrelationID will set the correlation ID of the message, based on AsyncAPI spec
func (msg *PongMessage) SetCorrelationID(id string) {
	msg.Headers.CorrelationId = &id
}

// SetAsResponseFrom will correlate the message with the one passed in parameter.
// It will assign the 'req' message correlation ID to the message correlation ID,
// both specified in AsyncAPI spec.
func (msg *PongMessage) SetAsResponseFrom(req MessageWithCorrelationID) {
	id := req.CorrelationID()
	msg.Headers.CorrelationId = &id
}

const (
	// PingChannelPath is the constant representing the 'PingChannel' channel path.
	PingChannelPath = "ping.v3"
	// PongChannelPath is the constant representing the 'PongChannel' channel path.
	PongChannelPath = "pong.v3"
)

// ChannelsPaths is an array of all channels paths
var ChannelsPaths = []string{
	PingChannelPath,
	PongChannelPath,
}
