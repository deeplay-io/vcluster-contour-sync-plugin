package syncers

import (
	"reflect"
	"testing"

	"github.com/loft-sh/vcluster-sdk/translate"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestTranslateHTTPProxySpec(t *testing.T) {
	translate.Suffix = "myvcluster"

	vSpec := map[string]interface{}{
		"ingressClassName": "contour-default",
		"virtualhost": map[string]interface{}{
			"fqdn": "app.example.org",
			"tls": map[string]interface{}{
				"secretName": "app-tls",
				"clientValidation": map[string]interface{}{
					"caSecret": "app-ca",
				},
			},
			"authorization": map[string]interface{}{
				"extensionRef": map[string]interface{}{
					"name": "auth-server",
				},
			},
			// unknown to Contour 1.21: must survive translation untouched
			"jwtProviders": []interface{}{
				map[string]interface{}{
					"name":    "provider-1",
					"default": true,
					"issuer":  "https://idp.example.org",
					"remoteJWKS": map[string]interface{}{
						"uri": "https://idp.example.org/jwks.json",
					},
				},
			},
		},
		"routes": []interface{}{
			map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{"prefix": "/api"},
					map[string]interface{}{
						"header": map[string]interface{}{
							"name": "authorization",
							// unknown to Contour 1.21: must survive translation untouched
							"regex": "Bearer sgm_.*",
						},
					},
				},
				"services": []interface{}{
					map[string]interface{}{
						"name": "app",
						"port": int64(8080),
					},
				},
				// unknown to Contour 1.21: must survive translation untouched
				"jwtVerificationPolicy": map[string]interface{}{
					"require": "provider-1",
				},
			},
		},
		"includes": []interface{}{
			map[string]interface{}{
				"name": "child-proxy",
			},
		},
	}
	vSpecCopy := runtime.DeepCopyJSON(vSpec)

	pSpec := translateHTTPProxySpec("app-ns", "host-ns", vSpec)

	if !reflect.DeepEqual(vSpec, vSpecCopy) {
		t.Errorf("input spec was mutated")
	}

	assertPath(t, pSpec, "app-tls-x-app-ns-x-myvcluster", "virtualhost", "tls", "secretName")
	assertPath(t, pSpec, "app-ca-x-app-ns-x-myvcluster", "virtualhost", "tls", "clientValidation", "caSecret")
	assertPath(t, pSpec, "auth-server-x-app-ns-x-myvcluster", "virtualhost", "authorization", "extensionRef", "name")
	assertPath(t, pSpec, "host-ns", "virtualhost", "authorization", "extensionRef", "namespace")

	route := asMap(asSlice(pSpec["routes"])[0])
	if got := asMap(asSlice(route["services"])[0])["name"]; got != "app-x-app-ns-x-myvcluster" {
		t.Errorf("route service name = %v", got)
	}
	if got := asMap(asSlice(pSpec["includes"])[0])["name"]; got != "child-proxy-x-app-ns-x-myvcluster" {
		t.Errorf("include name = %v", got)
	}

	// fields unknown to older Contour APIs must be passed through verbatim
	if jwt := asSlice(asMap(pSpec["virtualhost"])["jwtProviders"]); len(jwt) != 1 {
		t.Fatalf("jwtProviders lost: %v", jwt)
	} else if !reflect.DeepEqual(jwt[0], asSlice(asMap(vSpec["virtualhost"])["jwtProviders"])[0]) {
		t.Errorf("jwtProviders changed: %v", jwt[0])
	}
	header := asMap(asMap(asSlice(route["conditions"])[1])["header"])
	if header["regex"] != "Bearer sgm_.*" {
		t.Errorf("header regex lost: %v", header)
	}
	if !reflect.DeepEqual(route["jwtVerificationPolicy"], map[string]interface{}{"require": "provider-1"}) {
		t.Errorf("jwtVerificationPolicy lost: %v", route["jwtVerificationPolicy"])
	}
}

func TestTranslateExtensionServiceSpec(t *testing.T) {
	translate.Suffix = "myvcluster"

	pSpec := translateExtensionServiceSpec("app-ns", map[string]interface{}{
		"protocol": "h2",
		"services": []interface{}{
			map[string]interface{}{
				"name": "auth",
				"port": int64(9443),
			},
		},
	})

	if got := asMap(asSlice(pSpec["services"])[0])["name"]; got != "auth-x-app-ns-x-myvcluster" {
		t.Errorf("service name = %v", got)
	}
	if pSpec["protocol"] != "h2" {
		t.Errorf("protocol lost: %v", pSpec["protocol"])
	}
}

func assertPath(t *testing.T, spec map[string]interface{}, want interface{}, path ...string) {
	t.Helper()
	cur := spec
	for i, p := range path[:len(path)-1] {
		cur = asMap(cur[p])
		if cur == nil {
			t.Fatalf("path %v missing at %q", path, path[i])
		}
	}
	if got := cur[path[len(path)-1]]; !reflect.DeepEqual(got, want) {
		t.Errorf("%v = %v, want %v", path, got, want)
	}
}
