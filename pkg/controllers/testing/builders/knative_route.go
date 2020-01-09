/*
Copyright 2019 the original author or authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builders

import (
	"fmt"

	"github.com/projectriff/system/pkg/apis"
	knativeservingv1 "github.com/projectriff/system/pkg/apis/thirdparty/knative/serving/v1"
)

type knativeRoute struct {
	target *knativeservingv1.Route
}

func KnativeRoute(seed ...*knativeservingv1.Route) *knativeRoute {
	var target *knativeservingv1.Route
	switch len(seed) {
	case 0:
		target = &knativeservingv1.Route{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &knativeRoute{
		target: target,
	}
}

func (b *knativeRoute) deepCopy() *knativeRoute {
	return KnativeRoute(b.target.DeepCopy())
}

func (b *knativeRoute) Build() *knativeservingv1.Route {
	return b.deepCopy().target
}

func (b *knativeRoute) Mutate(m func(*knativeservingv1.Route)) *knativeRoute {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *knativeRoute) NamespaceName(namespace, name string) *knativeRoute {
	return b.Mutate(func(route *knativeservingv1.Route) {
		route.ObjectMeta.Namespace = namespace
		route.ObjectMeta.Name = name
	})
}

func (b *knativeRoute) ObjectMeta(nf func(ObjectMeta)) *knativeRoute {
	return b.Mutate(func(route *knativeservingv1.Route) {
		omf := objectMeta(route.ObjectMeta)
		nf(omf)
		route.ObjectMeta = omf.Build()
	})
}

func (b *knativeRoute) Traffic(traffic ...knativeservingv1.TrafficTarget) *knativeRoute {
	return b.Mutate(func(route *knativeservingv1.Route) {
		route.Spec.Traffic = traffic
	})
}

func (b *knativeRoute) StatusConditions(conditions ...*condition) *knativeRoute {
	return b.Mutate(func(route *knativeservingv1.Route) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Build()
		}
		route.Status.Conditions = c
	})
}

func (b *knativeRoute) StatusReady() *knativeRoute {
	return b.StatusConditions(
		Condition().Type(knativeservingv1.RouteConditionReady).True(),
	)
}

func (b *knativeRoute) StatusObservedGeneration(generation int64) *knativeRoute {
	return b.Mutate(func(route *knativeservingv1.Route) {
		route.Status.ObservedGeneration = generation
	})
}

func (b *knativeRoute) StatusAddressURL(url string) *knativeRoute {
	return b.Mutate(func(route *knativeservingv1.Route) {
		route.Status.Address = &apis.Addressable{
			URL: url,
		}
	})
}

func (b *knativeRoute) StatusURL(url string) *knativeRoute {
	return b.Mutate(func(route *knativeservingv1.Route) {
		route.Status.URL = url
	})
}
