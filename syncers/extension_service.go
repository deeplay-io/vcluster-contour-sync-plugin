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

var extensionServiceGVK = schema.GroupVersionKind{Group: "projectcontour.io", Version: "v1alpha1", Kind: "ExtensionService"}

func NewExtensionServiceSyncer(ctx *synccontext.RegisterContext) syncer.Base {
	return &extensionServiceSyncer{
		NamespacedTranslator: translator.NewNamespacedTranslator(ctx, "extensionservice", newUnstructuredWithGVK(extensionServiceGVK)),
	}
}

type extensionServiceSyncer struct {
	translator.NamespacedTranslator
}

var _ syncer.Initializer = &extensionServiceSyncer{}

func (s *extensionServiceSyncer) Init(ctx *synccontext.RegisterContext) error {
	return translate.EnsureCRDFromPhysicalCluster(ctx.Context, ctx.PhysicalManager.GetConfig(), ctx.VirtualManager.GetConfig(), extensionServiceGVK)
}

func (s *extensionServiceSyncer) SyncDown(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	vExtensionService := vObj.(*unstructured.Unstructured)
	pExtensionService := s.TranslateMetadata(vObj).(*unstructured.Unstructured)
	setSpec(pExtensionService, translateExtensionServiceSpec(vExtensionService.GetNamespace(), getSpec(vExtensionService)))
	return s.SyncDownCreate(ctx, vObj, pExtensionService)
}

func (s *extensionServiceSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	vExtensionService := vObj.(*unstructured.Unstructured)
	pExtensionService := pObj.(*unstructured.Unstructured)

	if !equality.Semantic.DeepEqual(getStatus(vExtensionService), getStatus(pExtensionService)) {
		newExtensionService := vExtensionService.DeepCopy()
		setStatus(newExtensionService, getStatus(pExtensionService))
		ctx.Log.Infof("update virtual extensionservice %s/%s, because status is out of sync", vExtensionService.GetNamespace(), vExtensionService.GetName())
		printChanges(vExtensionService, newExtensionService, ctx.Log)
		err := ctx.VirtualClient.Status().Update(ctx.Context, newExtensionService)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	updated := s.translateUpdate(pExtensionService, vExtensionService)
	if updated != nil {
		printChanges(pObj, updated, ctx.Log)
	}

	return s.SyncDownUpdate(ctx, vObj, updated)
}

func (s *extensionServiceSyncer) translateUpdate(pObj, vObj *unstructured.Unstructured) *unstructured.Unstructured {
	var updated *unstructured.Unstructured

	translatedSpec := translateExtensionServiceSpec(vObj.GetNamespace(), getSpec(vObj))
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

func translateExtensionServiceSpec(namespace string, vSpec map[string]interface{}) map[string]interface{} {
	if vSpec == nil {
		return nil
	}
	retSpec := runtime.DeepCopyJSON(vSpec)

	for _, s := range asSlice(retSpec["services"]) {
		service := asMap(s)
		if service == nil {
			continue
		}
		if name, _ := service["name"].(string); name != "" {
			service["name"] = translate.PhysicalName(name, namespace)
		}
	}

	return retSpec
}
