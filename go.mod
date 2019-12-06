module github.com/projectriff/system

go 1.13

require (
	contrib.go.opencensus.io/exporter/prometheus v0.1.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.12.8 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.3.1
	github.com/google/go-containerregistry v0.0.0-20191202175804-2ce3ea99b462 // indirect
	github.com/jetstack/cert-manager v0.9.1 // old version required by Knative testing package
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	istio.io/client-go v0.0.0-20191119175647-1aefa51f7583 // indirect
	// equivelent of kubernetes-1.16.3 tag for each k8s.io repo
	k8s.io/api v0.0.0-20191114100352-16d7abae0d2a
	k8s.io/apimachinery v0.0.0-20191028221656-72ed19daf4bb
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.0.0-20191114215150-2a85f169f05f
	knative.dev/caching v0.0.0-20191203215637-c97a7bc3ee60 // indirect
	knative.dev/pkg v0.0.0-20191203221237-94a34e416c44 // release-0.11 branch required by Knative testing package
	knative.dev/serving v0.10.1-0.20191205012937-552067c564b9 // new version required to resolve istio API dependency
	sigs.k8s.io/controller-runtime v0.4.0
)
