/*
 * Copyright (c) 2019 VMware, Inc. All Rights Reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package octant

import (
	"context"

	"github.com/vmware/octant/pkg/action"
)

// OctantClient is an OctantClient.
type OctantClient interface {
	Send(event Event)
	ID() string
}

// ClientRequestHandler is a client request.
type ClientRequestHandler struct {
	RequestType string
	Handler     func(ctx context.Context, state State, payload action.Payload, s OctantClient) error
}
