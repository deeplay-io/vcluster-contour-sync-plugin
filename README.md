## Contour Sync Plugin

This plugin syncs [Contour](https://projectcontour.io/) resources from the virtual cluster to the host cluster. It expects that Contour CRDs were already installed in the host cluster.

## Using the Plugin in vcluster

To use the plugin, create a new vcluster with the `plugin.yaml`:

```
vcluster create my-vcluster -n my-vcluster -f https://raw.githubusercontent.com/deeplay-io/vcluster-contour-sync-plugin/main/plugin.yaml
```

This will create a new vcluster with the plugin installed.

## Building the Plugin

To just build the plugin image and push it to the registry, run:

```
# Build
docker build . -t docker.io/aikoven/vcluster-contour-sync-plugin:0.1.0

# Push
docker push docker.io/aikoven/vcluster-contour-sync-plugin:0.1.0
```

Then exchange the image in the `plugin.yaml`
