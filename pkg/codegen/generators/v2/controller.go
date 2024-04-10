package generatorv2

import (
	"bytes"

	asyncapi "github.com/hound672/asyncapi-codegen/pkg/asyncapi/v2"
	"github.com/hound672/asyncapi-codegen/pkg/codegen/generators"
)

// ControllerGenerator is a code generator for controllers that will turn an
// asyncapi specification into controller golang code.
type ControllerGenerator struct {
	MethodCount       uint
	SubscribeChannels map[string]*asyncapi.Channel
	PublishChannels   map[string]*asyncapi.Channel
	Prefix            string
	Version           string
}

// NewControllerGenerator will create a new controller code generator.
func NewControllerGenerator(side generators.Side, spec asyncapi.Specification) ControllerGenerator {
	var gen ControllerGenerator

	// Get subscription methods count based on publish/subscribe count
	publishCount, subscribeCount := spec.GetPublishSubscribeCount()
	if side == generators.SideIsApplication {
		gen.MethodCount = publishCount
	} else {
		gen.MethodCount = subscribeCount
	}

	// Get channels based on publish/subscribe
	gen.SubscribeChannels = make(map[string]*asyncapi.Channel)
	gen.PublishChannels = make(map[string]*asyncapi.Channel)
	for name, channel := range spec.Channels {
		// Add channel to subscribe channels based on channel content and side
		if isSubscribeChannel(side, channel) {
			gen.SubscribeChannels[name] = channel
		}

		// Add channel to publish channels based on channel content and side
		if isPublishChannel(side, channel) {
			gen.PublishChannels[name] = channel
		}
	}

	// Set generation name
	if side == generators.SideIsApplication {
		gen.Prefix = "App"
	} else {
		gen.Prefix = "User"
	}

	// Set version
	gen.Version = spec.Info.Version

	return gen
}

func isSubscribeChannel(side generators.Side, channel *asyncapi.Channel) bool {
	switch {
	case side == generators.SideIsApplication && channel.Publish != nil:
		return true
	case side == generators.SideIsUser && channel.Subscribe != nil:
		return true
	default:
		return false
	}
}

func isPublishChannel(side generators.Side, channel *asyncapi.Channel) bool {
	switch {
	case side == generators.SideIsApplication && channel.Subscribe != nil:
		return true
	case side == generators.SideIsUser && channel.Publish != nil:
		return true
	default:
		return false
	}
}

// Generate will generate the controller code.
func (asg ControllerGenerator) Generate() (string, error) {
	tmplt, err := loadTemplate(
		controllerTemplatePath,
		schemaDefinitionTemplatePath,
		schemaNameTemplatePath,
		messageTemplatePath,
	)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err := tmplt.Execute(buf, asg); err != nil {
		return "", err
	}

	return buf.String(), nil
}
