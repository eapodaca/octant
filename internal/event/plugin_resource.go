package event

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/octant/internal/config"
	"github.com/vmware/octant/internal/octant"
)

type PluginResourceGeneratorOption func(generator *PluginResourceGenerator)

// PluginResourceGenerator generates kube contexts for the front end.
type PluginResourceGenerator struct {
	DashConfig config.Dash
}

var _ octant.Generator = (*PluginResourceGenerator)(nil)

func NewPluginResourcesGenerator(dashConfig config.Dash, options ...PluginResourceGeneratorOption) *PluginResourceGenerator {
	prg := &PluginResourceGenerator{
		DashConfig: dashConfig,
	}

	for _, option := range options {
		option(prg)
	}

	return prg
}

func (g *PluginResourceGenerator) listResources(ctx context.Context, mimeType string) []string {
	pm := g.DashConfig.PluginManager()
	resources, err := pm.PluginWebResourcesByType(ctx, mimeType)
	if err != nil {
		return make([]string, 0)
	}
	result := make([]string, len(resources))
	for _, resource := range resources {
		path := fmt.Sprintf("/api/assets/plugin/%s/path/%s", resource.PluginName, resource.Path)
		result = append(result, path)
	}
	return result
}

// Event generate an event for plugin resources
func (g *PluginResourceGenerator) Event(ctx context.Context) (octant.Event, error) {
	css := g.listResources(ctx, "text/css")
	js := g.listResources(ctx, "application/javascript")

	e := octant.Event{
		Type: octant.EventTypeWebResources,
		Data: map[string]interface{}{
			"resources": map[string]interface{}{
				"css": css,
				"js":  js,
			},
		},
	}

	return e, nil
}

func (PluginResourceGenerator) ScheduleDelay() time.Duration {
	return DefaultScheduleDelay
}

func (PluginResourceGenerator) Name() string {
	return "webResources"
}