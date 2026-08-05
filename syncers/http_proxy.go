package syncers

import (
	"github.com/loft-sh/vcluster-sdk/syncer"
	synccontext "github.com/loft-sh/vcluster-sdk/syncer/context"
	"github.com/loft-sh/vcluster-sdk/syncer/translator"
	"github.com/loft-sh/vcluster-sdk/translate"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The syncer works with unstructured objects instead of the typed Contour API
// so that fields unknown to the compiled-in API version are never dropped
// during translation. Only the fields that reference other Kubernetes objects
// by name are rewritten; everything else is passed through verbatim.
var httpProxyGVK = schema.GroupVersionKind{Group: "projectcontour.io", Version: "v1", Kind: "HTTPProxy"}

func NewHTTPProxySyncer(ctx *synccontext.RegisterContext) syncer.Base {
	return &httpProxySyncer{
		NamespacedTranslator: translator.NewNamespacedTranslator(ctx, "httpproxy", newUnstructuredWithGVK(httpProxyGVK)),
	}
}

type httpProxySyncer struct {
	translator.NamespacedTranslator
}

var _ syncer.Initializer = &httpProxySyncer{}

func (s *httpProxySyncer) Init(ctx *synccontext.RegisterContext) error {
	return translate.EnsureCRDFromPhysicalCluster(ctx.Context, ctx.PhysicalManager.GetConfig(), ctx.VirtualManager.GetConfig(), httpProxyGVK)
}

func (s *httpProxySyncer) SyncDown(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	vHTTPProxy := vObj.(*unstructured.Unstructured)
	pHTTPProxy := s.TranslateMetadata(vObj).(*unstructured.Unstructured)
	setSpec(pHTTPProxy, translateHTTPProxySpec(vHTTPProxy.GetNamespace(), pHTTPProxy.GetNamespace(), getSpec(vHTTPProxy)))
	return s.SyncDownCreate(ctx, vObj, pHTTPProxy)
}

func (s *httpProxySyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	vHTTPProxy := vObj.(*unstructured.Unstructured)
	pHTTPProxy := pObj.(*unstructured.Unstructured)

	if !equality.Semantic.DeepEqual(getStatus(vHTTPProxy), getStatus(pHTTPProxy)) {
		newHTTPProxy := vHTTPProxy.DeepCopy()
		setStatus(newHTTPProxy, getStatus(pHTTPProxy))
		ctx.Log.Infof("update virtual httpproxy %s/%s, because status is out of sync", vHTTPProxy.GetNamespace(), vHTTPProxy.GetName())
		printChanges(vHTTPProxy, newHTTPProxy, ctx.Log)
		err := ctx.VirtualClient.Status().Update(ctx.Context, newHTTPProxy)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	updated := s.translateUpdate(pHTTPProxy, vHTTPProxy)
	if updated != nil {
		printChanges(pObj, updated, ctx.Log)
	}

	return s.SyncDownUpdate(ctx, vObj, updated)
}

func (s *httpProxySyncer) translateUpdate(pObj, vObj *unstructured.Unstructured) *unstructured.Unstructured {
	var updated *unstructured.Unstructured

	translatedSpec := translateHTTPProxySpec(vObj.GetNamespace(), pObj.GetNamespace(), getSpec(vObj))
	if !equality.Semantic.DeepEqual(translatedSpec, getSpec(pObj)) {
		updated = newIfNil(updated, pObj)
		setSpec(updated, translatedSpec)
	}

	_, translatedAnnotations, translatedLabels := s.TranslateMetadataUpdate(vObj, pObj)

	if !equality.Semantic.DeepEqual(translatedAnnotations, pObj.GetAnnotations()) || !equality.Semantic.DeepEqual(translatedLabels, pObj.GetLabels()) {
		updated = newIfNil(updated, pObj)
		updated.SetAnnotations(translatedAnnotations)
		updated.SetLabels(translatedLabels)
	}

	return updated
}

func translateHTTPProxySpec(namespace string, physicalNamespace string, vSpec map[string]interface{}) map[string]interface{} {
	if vSpec == nil {
		return nil
	}
	retSpec := runtime.DeepCopyJSON(vSpec)

	if virtualHost := asMap(retSpec["virtualhost"]); virtualHost != nil {
		if tls := asMap(virtualHost["tls"]); tls != nil {
			if secretName, _ := tls["secretName"].(string); secretName != "" {
				tls["secretName"] = translate.PhysicalName(secretName, namespace)

				if clientValidation := asMap(tls["clientValidation"]); clientValidation != nil {
					if caSecret, _ := clientValidation["caSecret"].(string); caSecret != "" {
						clientValidation["caSecret"] = translate.PhysicalName(caSecret, namespace)
					}
				}
			}
		}

		if authorization := asMap(virtualHost["authorization"]); authorization != nil {
			if extensionRef := asMap(authorization["extensionRef"]); extensionRef != nil {
				if name, _ := extensionRef["name"].(string); name != "" {
					refNamespace, _ := extensionRef["namespace"].(string)
					if refNamespace == "" {
						refNamespace = namespace
					}

					extensionRef["name"] = translate.PhysicalName(name, refNamespace)
					extensionRef["namespace"] = physicalNamespace
				}
			}
		}
	}

	for _, r := range asSlice(retSpec["routes"]) {
		route := asMap(r)
		if route == nil {
			continue
		}
		for _, s := range asSlice(route["services"]) {
			service := asMap(s)
			if service == nil {
				continue
			}
			if name, _ := service["name"].(string); name != "" {
				service["name"] = translate.PhysicalName(name, namespace)
			}
		}
	}

	for _, i := range asSlice(retSpec["includes"]) {
		include := asMap(i)
		if include == nil {
			continue
		}
		if name, _ := include["name"].(string); name != "" {
			include["name"] = translate.PhysicalName(name, namespace)
		}
	}

	return retSpec
}
