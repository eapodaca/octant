package api

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/vmware/octant/internal/log"
)

func pluginAssetsHandler(ctx context.Context) http.HandlerFunc {
	logger := log.From(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		plugin := vars["plugin"]
		path := vars["path"]

		logger.Infof("getting asset [%s]:%s", plugin, path)

	}
}
