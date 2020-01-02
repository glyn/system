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

package factories

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/projectriff/system/pkg/apis"
	knativeservingv1 "github.com/projectriff/system/pkg/apis/thirdparty/knative/serving/v1"
)

type knativeService struct {
	target *knativeservingv1.Service
}

func KnativeService(seed ...*knativeservingv1.Service) *knativeService {
	var target *knativeservingv1.Service
	switch len(seed) {
	case 0:
		target = &knativeservingv1.Service{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &knativeService{
		target: target,
	}
}

func (f *knativeService) deepCopy() *knativeService {
	return KnativeService(f.target.DeepCopy())
}

func (f *knativeService) Get() *knativeservingv1.Service {
	return f.deepCopy().target
}

func (f *knativeService) Mutate(m func(*knativeservingv1.Service)) *knativeService {
	f = f.deepCopy()
	m(f.target)
	return f
}

func (f *knativeService) NamespaceName(namespace, name string) *knativeService {
	return f.Mutate(func(service *knativeservingv1.Service) {
		service.ObjectMeta.Namespace = namespace
		service.ObjectMeta.Name = name
	})
}

func (f *knativeService) ObjectMeta(nf func(ObjectMeta)) *knativeService {
	return f.Mutate(func(service *knativeservingv1.Service) {
		omf := objectMeta(service.ObjectMeta)
		nf(omf)
		service.ObjectMeta = omf.Get()
	})
}

func (f *knativeService) PodTemplateSpec(nf func(PodTemplateSpec)) *knativeService {
	return f.Mutate(func(service *knativeservingv1.Service) {
		ptsf := podTemplateSpec(
			// convert RevisionTemplateSpec into PodTemplateSpec
			corev1.PodTemplateSpec{
				ObjectMeta: service.Spec.Template.ObjectMeta,
				Spec:       service.Spec.Template.Spec.PodSpec,
			},
		)
		nf(ptsf)
		template := ptsf.Get()
		// update RevisionTemplateSpec with PodTemplateSpec managed fields
		service.Spec.Template.ObjectMeta = template.ObjectMeta
		service.Spec.Template.Spec.PodSpec = template.Spec
	})
}

func (f *knativeService) UserContainer(cb func(*corev1.Container)) *knativeService {
	return f.PodTemplateSpec(func(pts PodTemplateSpec) {
		pts.ContainerNamed("user-container", cb)
	})
}

func (f *knativeService) StatusConditions(conditions ...apis.Condition) *knativeService {
	return f.Mutate(func(service *knativeservingv1.Service) {
		service.Status.Conditions = conditions
	})
}

func (f *knativeService) StatusObservedGeneration(generation int64) *knativeService {
	return f.Mutate(func(service *knativeservingv1.Service) {
		service.Status.ObservedGeneration = generation
	})
}
