package api

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/vmware/octant/internal/config"
	"github.com/vmware/octant/internal/event"
	"github.com/vmware/octant/internal/octant"
)

// PluginManagerOption is an option for configuring PluginManager.
type PluginManagerOption func(manager *PluginManager)

// PluginEventGenerateFunc is a function which generates a context event.
type PluginEventGenerateFunc func(ctx context.Context, state octant.State) (octant.Event, error)

// WithPluginEventGenerator sets the context generator.
func WithPluginEventGenerator(fn PluginEventGenerateFunc) PluginManagerOption {
	return func(manager *PluginManager) {
		manager.pluginEventGenerateFunc = fn
	}
}

// WithPluginEventGeneratorPoll generates the poller.
func WithPluginEventGeneratorPoll(poller Poller) PluginManagerOption {
	return func(manager *PluginManager) {
		manager.poller = poller
	}
}

// PluginManager manages context.
type PluginManager struct {
	dashConfig              config.Dash
	pluginEventGenerateFunc PluginEventGenerateFunc
	poller                  Poller
}

var _ StateManager = (*PluginManager)(nil)

// NewPluginManager creates an instances of PluginManager.
func NewPluginManager(dashConfig config.Dash, options ...PluginManagerOption) *PluginManager {
	cm := &PluginManager{
		dashConfig: dashConfig,
		poller:     NewInterruptiblePoller("pluginresources"),
	}

	cm.pluginEventGenerateFunc = cm.generatePluginEvents

	for _, option := range options {
		option(cm)
	}

	return cm
}

// Handlers returns a slice of handlers.
func (c *PluginManager) Handlers() []octant.ClientRequestHandler {
	return nil
}

// Start starts the manager.
func (c *PluginManager) Start(ctx context.Context, state octant.State, s octant.OctantClient) {
	c.poller.Run(ctx, nil, c.runUpdate(state, s), event.DefaultScheduleDelay)
}

func (c *PluginManager) runUpdate(state octant.State, s octant.OctantClient) PollerFunc {
	var previous []byte

	logger := c.dashConfig.Logger()
	return func(ctx context.Context) bool {
		ev, err := c.pluginEventGenerateFunc(ctx, state)
		if err != nil {
			logger.WithErr(err).Errorf("generate contexts")
		}

		if ctx.Err() == nil {
			cur, err := json.Marshal(ev)
			if err != nil {
				logger.WithErr(err).Errorf("unable to marshal context")
				return false
			}

			if bytes.Compare(previous, cur) != 0 {
				previous = cur
				s.Send(ev)
			}
		}

		return false
	}
}

func (c *PluginManager) generatePluginEvents(ctx context.Context, state octant.State) (octant.Event, error) {
	generator, err := c.initGenerator(state)
	if err != nil {
		return octant.Event{}, err
	}
	return generator.Event(ctx)
}

func (c *PluginManager) initGenerator(state octant.State) (*event.PluginResourceGenerator, error) {
	return event.NewPluginResourcesGenerator(c.dashConfig), nil
}
