package versioning

import (
	"context"
	"fmt"
	"sync"

	"github.com/lerenn/asyncapi-codegen/pkg/extensions"
)

type brokerSubscription struct {
	channelName string
	messages    chan extensions.BrokerMessage
	cancel      chan any
	parent      *Wrapper

	versionsChannels map[string]versionSubcription
	versionsMutex    sync.Mutex
}

func newBrokerSubscription(
	channelName string,
	messages chan extensions.BrokerMessage,
	cancel chan any,
	parent *Wrapper,
) brokerSubscription {
	return brokerSubscription{
		channelName:      channelName,
		messages:         messages,
		cancel:           cancel,
		parent:           parent,
		versionsChannels: make(map[string]versionSubcription),
	}
}

func (bs *brokerSubscription) createVersionListener(version string) (versionSubcription, error) {
	// Lock the versions to avoid conflict
	bs.versionsMutex.Lock()
	defer bs.versionsMutex.Unlock()

	// Check if the version doesn't exist already
	_, exists := bs.versionsChannels[version]
	if exists {
		return versionSubcription{}, extensions.ErrAlreadySubscribedChannel
	}

	// Create the channels necessary
	cbv := newVersionSubscription(version, bs)
	bs.versionsChannels[version] = cbv
	defer cbv.launchListener()

	return cbv, nil
}

func (bs *brokerSubscription) removeVersionListener(vs *versionSubcription) {
	// Lock the versions to avoid conflict
	bs.versionsMutex.Lock()
	defer bs.versionsMutex.Unlock()

	// Cleanup the channelsByVersion when leaving
	//
	// NOTE: this is important to make it cleanup at the end of this function as
	// it should be cleanup AFTER the broker have been stopped (in case it was
	// the last version listener), in order to let the caller knows that everything
	// was cleaned up properly.
	defer vs.closeChannels()

	// Remove the version from the channelsByBroker
	delete(bs.versionsChannels, vs.version)

	// Lock the channels to avoid conflict
	bs.parent.channelsMutex.Lock()
	defer bs.parent.channelsMutex.Unlock()

	// If there is still version channels, do nothing
	if len(bs.versionsChannels) > 0 {
		return
	}

	// Otherwise cancel the broker listener and wait for its closure
	bs.cancel <- true
	<-bs.cancel

	// Then delete the channelsByBroker from the Version Switch Wrapper
	delete(bs.parent.channels, bs.channelName)
}

func (bs *brokerSubscription) launchListener(ctx context.Context) {
	go func() {
		for {
			// Wait for new messages
			msg, open := <-bs.messages
			if !open {
				break
			}

			// Get the version from the message
			bVersion, exists := msg.Headers[bs.parent.versionHeaderKey]
			version := string(bVersion)

			// Add default version if none is specified
			if !exists || version == "" {
				// If there is a default version activated, then go on with it
				if bs.parent.defaultVersion != nil {
					version = *bs.parent.defaultVersion
				} else {
					ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, msg)
					bs.parent.logger.Error(ctx, "no version in the message and no default version")
					continue
				}
			}

			// Lock the versions to avoid conflict
			bs.versionsMutex.Lock()

			// Get the correct channel based on the version
			ch, exists := bs.versionsChannels[version]
			if !exists {
				// Set context
				ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, msg)
				ctx = context.WithValue(ctx, extensions.ContextKeyIsVersion, version)

				// Log the error
				bs.parent.logger.Error(ctx, fmt.Sprintf("version %q is not registered", version))
				continue
			}

			// Unlock the versions
			bs.versionsMutex.Unlock()

			// Send the message to the correct channel
			ch.messages <- msg
		}
	}()
}