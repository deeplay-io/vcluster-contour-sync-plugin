package syncers

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newUnstructuredWithGVK(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return u
}

func getSpec(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	return spec
}

func setSpec(obj *unstructured.Unstructured, spec map[string]interface{}) {
	if spec == nil {
		delete(obj.Object, "spec")
		return
	}
	obj.Object["spec"] = spec
}

func getStatus(obj *unstructured.Unstructured) map[string]interface{} {
	status, _ := obj.Object["status"].(map[string]interface{})
	return status
}

func setStatus(obj *unstructured.Unstructured, status map[string]interface{}) {
	if status == nil {
		delete(obj.Object, "status")
		return
	}
	obj.Object["status"] = runtime.DeepCopyJSON(status)
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func asSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}

func newIfNil(updated, pObj *unstructured.Unstructured) *unstructured.Unstructured {
	if updated == nil {
		return pObj.DeepCopy()
	}
	return updated
}
