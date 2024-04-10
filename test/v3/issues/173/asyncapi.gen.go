// Package "issue173" provides primitives to interact with the AsyncAPI specification.
//
// Code generated by github.com/hound672/asyncapi-codegen version (devel) DO NOT EDIT.
package issue173

import (
	"encoding/json"
	"fmt"

	"github.com/hound672/asyncapi-codegen/pkg/extensions"
)

// AsyncAPIVersion is the version of the used AsyncAPI document
const AsyncAPIVersion = ""

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

// Type1MessageHeaders is a schema from the AsyncAPI specification required in messages
type Type1MessageHeaders struct {
	// Description: Correlation ID set by client
	CorrelationId *string `json:"correlation_id"`
}

// Type1MessagePayload is a schema from the AsyncAPI specification required in messages
type Type1MessagePayload struct{}

// Type1Message is the message expected for 'Type1Message' channel.
type Type1Message struct {
	// Headers will be used to fill the message headers
	Headers Type1MessageHeaders

	// Payload will be inserted in the message payload
	Payload Type1MessagePayload
}

func NewType1Message() Type1Message {
	var msg Type1Message

	return msg
}

// newType1MessageFromBrokerMessage will fill a new Type1Message with data from generic broker message
func newType1MessageFromBrokerMessage(bMsg extensions.BrokerMessage) (Type1Message, error) {
	var msg Type1Message

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

// toBrokerMessage will generate a generic broker message from Type1Message data
func (msg Type1Message) toBrokerMessage() (extensions.BrokerMessage, error) {
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

// Type2MessageHeaders is a schema from the AsyncAPI specification required in messages
type Type2MessageHeaders struct {
	// Description: Correlation ID set by client
	CorrelationId *string `json:"correlation_id"`
}

// Type2MessagePayload is a schema from the AsyncAPI specification required in messages
type Type2MessagePayload struct{}

// Type2Message is the message expected for 'Type2Message' channel.
type Type2Message struct {
	// Headers will be used to fill the message headers
	Headers Type2MessageHeaders

	// Payload will be inserted in the message payload
	Payload Type2MessagePayload
}

func NewType2Message() Type2Message {
	var msg Type2Message

	return msg
}

// newType2MessageFromBrokerMessage will fill a new Type2Message with data from generic broker message
func newType2MessageFromBrokerMessage(bMsg extensions.BrokerMessage) (Type2Message, error) {
	var msg Type2Message

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

// toBrokerMessage will generate a generic broker message from Type2Message data
func (msg Type2Message) toBrokerMessage() (extensions.BrokerMessage, error) {
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