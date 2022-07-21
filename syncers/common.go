package syncers

import (
	"os"

	"github.com/loft-sh/vcluster-sdk/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func printChanges(oldObject, newObject client.Object, log log.Logger) {
	if os.Getenv("DEBUG") == "true" {
		rawPatch, err := client.MergeFrom(oldObject).Data(newObject)
		if err == nil {
			log.Debugf("Updating object with: %v", string(rawPatch))
		}
	}
}
