package api

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/vmware/octant/internal/log"
	"github.com/vmware/octant/pkg/plugin"
)

func pluginAssetsHandler(ctx context.Context, pluginManager plugin.ManagerInterface) http.HandlerFunc {
	logger := log.From(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		plugin := vars["plugin"]
		path := vars["path"]

		logger.Infof("getting asset [%s]:%s", plugin, path)

		resource, err := pluginManager.PluginWebResource(ctx, plugin, path)

		if err != nil {
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(200)
		w.Header().Set("Content-Type", resource.MimeType)
		w.Write(resource.Content)
	}
}
