package syncers

import (
	"github.com/loft-sh/vcluster-sdk/plugin"
	"github.com/loft-sh/vcluster-sdk/syncer"
	synccontext "github.com/loft-sh/vcluster-sdk/syncer/context"
	"github.com/loft-sh/vcluster-sdk/syncer/translator"
	"github.com/loft-sh/vcluster-sdk/translate"
	projectcontourv1alpha1 "github.com/projectcontour/contour/apis/projectcontour/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	// Make sure our scheme is registered
	_ = projectcontourv1alpha1.AddToScheme(plugin.Scheme)
}

func NewExtensionServiceSyncer(ctx *synccontext.RegisterContext) syncer.Base {
	return &extensionServiceSyncer{
		NamespacedTranslator: translator.NewNamespacedTranslator(ctx, "extensionservice", &projectcontourv1alpha1.ExtensionService{}),
	}
}

type extensionServiceSyncer struct {
	translator.NamespacedTranslator
}

var _ syncer.Initializer = &extensionServiceSyncer{}

func (s *extensionServiceSyncer) Init(ctx *synccontext.RegisterContext) error {
	return translate.EnsureCRDFromPhysicalCluster(ctx.Context, ctx.PhysicalManager.GetConfig(), ctx.VirtualManager.GetConfig(), projectcontourv1alpha1.GroupVersion.WithKind("ExtensionService"))
}

func (s *extensionServiceSyncer) SyncDown(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	return s.SyncDownCreate(ctx, vObj, s.TranslateMetadata(vObj).(*projectcontourv1alpha1.ExtensionService))
}

func (s *extensionServiceSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	vExtensionService := vObj.(*projectcontourv1alpha1.ExtensionService)
	pExtensionService := pObj.(*projectcontourv1alpha1.ExtensionService)

	if !equality.Semantic.DeepEqual(vExtensionService.Status, pExtensionService.Status) {
		newExtensionService := vExtensionService.DeepCopy()
		newExtensionService.Status = pExtensionService.Status
		ctx.Log.Infof("update virtual extensionservice %s/%s, because status is out of sync", vExtensionService.Namespace, vExtensionService.Name)
		printChanges(vExtensionService, newExtensionService, ctx.Log)
		err := ctx.VirtualClient.Status().Update(ctx.Context, newExtensionService)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	newIngress := s.translateUpdate(pExtensionService, vExtensionService)
	if newIngress != nil {
		printChanges(pObj, newIngress, ctx.Log)
	}

	return s.SyncDownUpdate(ctx, vObj, newIngress)
}

func (s *extensionServiceSyncer) translate(vObj *projectcontourv1alpha1.ExtensionService) *projectcontourv1alpha1.ExtensionService {
	newExtensionService := s.TranslateMetadata(vObj).(*projectcontourv1alpha1.ExtensionService)
	newExtensionService.Spec = *translateExtensionServiceSpec(vObj.Namespace, &vObj.Spec)
	return newExtensionService
}

func (s *extensionServiceSyncer) translateUpdate(pObj, vObj *projectcontourv1alpha1.ExtensionService) *projectcontourv1alpha1.ExtensionService {
	var updated *projectcontourv1alpha1.ExtensionService

	translatedSpec := *translateExtensionServiceSpec(vObj.Namespace, &vObj.Spec)
	if !equality.Semantic.DeepEqual(translatedSpec, pObj.Spec) {
		updated = newExtensionServiceIfNil(updated, pObj)
		updated.Spec = translatedSpec
	}

	_, translatedAnnotations, translatedLabels := s.TranslateMetadataUpdate(vObj, pObj)

	if !equality.Semantic.DeepEqual(translatedAnnotations, pObj.GetAnnotations()) || !equality.Semantic.DeepEqual(translatedLabels, pObj.GetLabels()) {
		updated = newExtensionServiceIfNil(updated, pObj)
		updated.Annotations = translatedAnnotations
		updated.Labels = translatedLabels
	}

	return updated
}

func translateExtensionServiceSpec(namespace string, vSpec *projectcontourv1alpha1.ExtensionServiceSpec) *projectcontourv1alpha1.ExtensionServiceSpec {
	retSpec := vSpec.DeepCopy()

	for i, service := range retSpec.Services {
		if service.Name != "" {
			retSpec.Services[i].Name = translate.PhysicalName(service.Name, namespace)
		}
	}

	return retSpec
}

func newExtensionServiceIfNil(updated *projectcontourv1alpha1.ExtensionService, pObj *projectcontourv1alpha1.ExtensionService) *projectcontourv1alpha1.ExtensionService {
	if updated == nil {
		return pObj.DeepCopy()
	}
	return updated
}
