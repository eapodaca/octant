/*
 * Copyright (c) 2019 VMware, Inc. All Rights Reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package api

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/vmware/octant/internal/event"
	"github.com/vmware/octant/internal/log"
	"github.com/vmware/octant/internal/module"
	"github.com/vmware/octant/internal/octant"
	"github.com/vmware/octant/pkg/action"
	"github.com/vmware/octant/pkg/view/component"
)

const (
	RequestSetContentPath = "setContentPath"
	RequestSetNamespace   = "setNamespace"
	RequestSetQueryParams = "setQueryParams"
)

// ContentManagerOption is an option for configuring ContentManager.
type ContentManagerOption func(manager *ContentManager)

// ContentGenerateFunc is a function that generates content. It returns `rerun=true`
// if the action should be be immediately rerun.
type ContentGenerateFunc func(ctx context.Context, state octant.State) (component.ContentResponse, bool, error)

// WithContentGenerator configures the content generate function.
func WithContentGenerator(fn ContentGenerateFunc) ContentManagerOption {
	return func(manager *ContentManager) {
		manager.contentGenerateFunc = fn
	}
}

// WithContentGeneratorPoller configures the poller.
func WithContentGeneratorPoller(poller Poller) ContentManagerOption {
	return func(manager *ContentManager) {
		manager.poller = poller
	}
}

// ContentManager manages content for websockets.
type ContentManager struct {
	moduleManager       module.ManagerInterface
	logger              log.Logger
	contentGenerateFunc ContentGenerateFunc
	poller              Poller
}

// NewContentManager creates an instance of ContentManager.
func NewContentManager(moduleManager module.ManagerInterface, logger log.Logger, options ...ContentManagerOption) *ContentManager {
	cm := &ContentManager{
		moduleManager: moduleManager,
		logger:        logger,
		poller:        NewInterruptiblePoller(),
	}
	cm.contentGenerateFunc = cm.generateContent

	for _, option := range options {
		option(cm)
	}

	return cm
}

var _ StateManager = (*ContentManager)(nil)

// Start starts the manager.
func (cm *ContentManager) Start(ctx context.Context, state octant.State, s OctantClient) {
	updateContentPathCh := make(chan struct{}, 1)
	defer func() {
		close(updateContentPathCh)
	}()

	updateCancel := state.OnContentPathUpdate(func(_ string) {
		updateContentPathCh <- struct{}{}
	})
	defer updateCancel()

	cm.poller.Run(ctx, updateContentPathCh, cm.runUpdate(state, s), event.DefaultScheduleDelay)
}

func (cm *ContentManager) runUpdate(state octant.State, s OctantClient) PollerFunc {
	return func(ctx context.Context) bool {
		for {
			contentResponse, rerun, err := cm.contentGenerateFunc(ctx, state)
			if err != nil {
				cm.logger.
					WithErr(err).
					With("contentPath", state.GetContentPath()).
					Errorf("generate content")
				return false
			}

			if ctx.Err() == nil {
				s.Send(CreateContentEvent(contentResponse))
			}

			if !rerun {
				break
			}
		}

		return false
	}
}

func (cm *ContentManager) generateContent(ctx context.Context, state octant.State) (component.ContentResponse, bool, error) {
	contentPath := state.GetContentPath()
	if contentPath != "" {
		logger := cm.logger.With("contentPath", contentPath)

		now := time.Now()

		contentPath = strings.TrimPrefix(contentPath, "/content/")
		m, ok := cm.moduleManager.ModuleForContentPath(contentPath)
		if !ok {
			return component.EmptyContentResponse, false, errors.Errorf("unable to find module for content path %q", contentPath)
		}
		modulePath := strings.TrimPrefix(contentPath, m.Name())
		options := module.ContentOptions{
			LabelSet: FiltersToLabelSet(state.GetFilters()),
		}
		contentResponse, err := m.Content(ctx, modulePath, options)
		if err != nil {
			if nfe, ok := err.(notFound); ok && nfe.NotFound() {
				logger.Debugf("path not found, redirecting to parent")
				state.SetContentPath(notFoundRedirectPath(contentPath))
				return component.EmptyContentResponse, true, nil
			} else {
				return component.EmptyContentResponse, false, errors.Wrap(err, "generate content")
			}
		}

		logger.With(
			"elapsed", time.Since(now),
		).Debugf("generate content")
		return contentResponse, false, nil
	}

	return component.EmptyContentResponse, false, nil
}

// Handlers returns a slice of client request handlers.
func (cm *ContentManager) Handlers() []octant.ClientRequestHandler {
	return []octant.ClientRequestHandler{
		{
			RequestType: RequestSetContentPath,
			Handler:     cm.SetContentPath,
		},
		{
			RequestType: RequestSetNamespace,
			Handler:     cm.SetNamespace,
		},
		{
			RequestType: RequestSetQueryParams,
			Handler:     cm.SetQueryParams,
		},
	}
}

// SetQueryParams sets the current query params.
func (cm *ContentManager) SetQueryParams(state octant.State, payload action.Payload) error {
	if params, ok := payload["params"].(map[string]interface{}); ok {
		// handle filters
		if filters, ok := params["filters"]; ok {
			list, err := FiltersFromQueryParams(filters)
			if err != nil {
				return errors.Wrap(err, "extract filters from query params")
			}
			state.SetFilters(list)
		}
	}

	return nil
}

// SetNamespace sets the current namespace.
func (cm *ContentManager) SetNamespace(state octant.State, payload action.Payload) error {
	namespace, err := payload.String("namespace")
	if err != nil {
		return errors.Wrap(err, "extract namespace from payload")
	}
	state.SetNamespace(namespace)
	return nil
}

// SetContentPath sets the current content path.
func (cm *ContentManager) SetContentPath(state octant.State, payload action.Payload) error {
	contentPath, err := payload.String("contentPath")
	if err != nil {
		return errors.Wrap(err, "extract contentPath from payload")
	}
	state.SetContentPath(contentPath)
	return nil
}

type notFound interface {
	NotFound() bool
	Path() string
}

// CreateContentEvent creates a content event.
func CreateContentEvent(contentResponse component.ContentResponse) octant.Event {
	return octant.Event{
		Type: octant.EventTypeContent,
		Data: contentResponse,
	}
}
