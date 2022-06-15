package syncers

import (
	"os"

	"github.com/loft-sh/vcluster-sdk/log"
	"github.com/loft-sh/vcluster-sdk/plugin"
	"github.com/loft-sh/vcluster-sdk/syncer"
	synccontext "github.com/loft-sh/vcluster-sdk/syncer/context"
	"github.com/loft-sh/vcluster-sdk/syncer/translator"
	"github.com/loft-sh/vcluster-sdk/translate"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	// Make sure our scheme is registered
	_ = projectcontourv1.AddToScheme(plugin.Scheme)
}

func NewHTTPProxySyncer(ctx *synccontext.RegisterContext) syncer.Base {
	return &httpProxySyncer{
		NamespacedTranslator: translator.NewNamespacedTranslator(ctx, "httpproxy", &projectcontourv1.HTTPProxy{}),
	}
}

type httpProxySyncer struct {
	translator.NamespacedTranslator
}

var _ syncer.Initializer = &httpProxySyncer{}

func (s *httpProxySyncer) Init(ctx *synccontext.RegisterContext) error {
	return translate.EnsureCRDFromPhysicalCluster(ctx.Context, ctx.PhysicalManager.GetConfig(), ctx.VirtualManager.GetConfig(), projectcontourv1.GroupVersion.WithKind("HTTPProxy"))
}

func (s *httpProxySyncer) SyncDown(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	return s.SyncDownCreate(ctx, vObj, s.TranslateMetadata(vObj).(*projectcontourv1.HTTPProxy))
}

func (s *httpProxySyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	vHTTPProxy := vObj.(*projectcontourv1.HTTPProxy)
	pHTTPProxy := pObj.(*projectcontourv1.HTTPProxy)

	if !equality.Semantic.DeepEqual(vHTTPProxy.Status, pHTTPProxy.Status) {
		newHTTPPRoxy := vHTTPProxy.DeepCopy()
		newHTTPPRoxy.Status = pHTTPProxy.Status
		ctx.Log.Infof("update virtual httpproxy %s/%s, because status is out of sync", vHTTPProxy.Namespace, vHTTPProxy.Name)
		printChanges(vHTTPProxy, newHTTPPRoxy, ctx.Log)
		err := ctx.VirtualClient.Status().Update(ctx.Context, newHTTPPRoxy)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	newIngress := s.translateUpdate(pHTTPProxy, vHTTPProxy)
	if newIngress != nil {
		printChanges(pObj, newIngress, ctx.Log)
	}

	return s.SyncDownUpdate(ctx, vObj, newIngress)
}

func (s *httpProxySyncer) translate(vObj *projectcontourv1.HTTPProxy) *projectcontourv1.HTTPProxy {
	newHttpProxy := s.TranslateMetadata(vObj).(*projectcontourv1.HTTPProxy)
	newHttpProxy.Spec = *translateSpec(vObj.Namespace, &vObj.Spec)
	return newHttpProxy
}

func (s *httpProxySyncer) translateUpdate(pObj, vObj *projectcontourv1.HTTPProxy) *projectcontourv1.HTTPProxy {
	var updated *projectcontourv1.HTTPProxy

	translatedSpec := *translateSpec(vObj.Namespace, &vObj.Spec)
	if !equality.Semantic.DeepEqual(translatedSpec, pObj.Spec) {
		updated = newIfNil(updated, pObj)
		updated.Spec = translatedSpec
	}

	_, translatedAnnotations, translatedLabels := s.TranslateMetadataUpdate(vObj, pObj)

	if !equality.Semantic.DeepEqual(translatedAnnotations, pObj.GetAnnotations()) || !equality.Semantic.DeepEqual(translatedLabels, pObj.GetLabels()) {
		updated = newIfNil(updated, pObj)
		updated.Annotations = translatedAnnotations
		updated.Labels = translatedLabels
	}

	return updated
}

func translateSpec(namespace string, vSpec *projectcontourv1.HTTPProxySpec) *projectcontourv1.HTTPProxySpec {
	retSpec := vSpec.DeepCopy()

	if retSpec.VirtualHost != nil && retSpec.VirtualHost.TLS != nil && retSpec.VirtualHost.TLS.SecretName != "" {
		retSpec.VirtualHost.TLS.SecretName = translate.PhysicalName(retSpec.VirtualHost.TLS.SecretName, namespace)
	}

	for _, route := range retSpec.Routes {
		if route.Services != nil {
			for _, service := range route.Services {

				if service.Name != "" {
					service.Name = translate.PhysicalName(service.Name, namespace)
				}
			}
		}
	}

	return retSpec
}

func newIfNil(updated *projectcontourv1.HTTPProxy, pObj *projectcontourv1.HTTPProxy) *projectcontourv1.HTTPProxy {
	if updated == nil {
		return pObj.DeepCopy()
	}
	return updated
}

func printChanges(oldObject, newObject client.Object, log log.Logger) {
	if os.Getenv("DEBUG") == "true" {
		rawPatch, err := client.MergeFrom(oldObject).Data(newObject)
		if err == nil {
			log.Debugf("Updating object with: %v", string(rawPatch))
		}
	}
}
