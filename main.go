package main

import (
	"github.com/deeplay-io/vcluster-contour-sync-plugin/syncers"
	"github.com/loft-sh/vcluster-sdk/plugin"
)

func main() {
	ctx := plugin.MustInit()
	plugin.MustRegister(syncers.NewHTTPProxySyncer(ctx))
	plugin.MustRegister(syncers.NewExtensionServiceSyncer(ctx))
	plugin.MustStart()
}
