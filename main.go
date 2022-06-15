package main

import (
	"github.com/deeplay-io/vcluster-contour-sync-plugin/syncers"
	"github.com/loft-sh/vcluster-sdk/plugin"
)

func main() {
	ctx := plugin.MustInit("contour-sync-plugin")
	plugin.MustRegister(syncers.NewHTTPProxySyncer(ctx))
	plugin.MustStart()
}
