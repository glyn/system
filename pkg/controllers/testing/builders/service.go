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

	corev1 "k8s.io/api/core/v1"
)

type service struct {
	target *corev1.Service
}

func Service(seed ...*corev1.Service) *service {
	var target *corev1.Service
	switch len(seed) {
	case 0:
		target = &corev1.Service{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &service{
		target: target,
	}
}

func (b *service) deepCopy() *service {
	return Service(b.target.DeepCopy())
}

func (b *service) Build() *corev1.Service {
	return b.deepCopy().target
}

func (b *service) Mutate(m func(*corev1.Service)) *service {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *service) NamespaceName(namespace, name string) *service {
	return b.Mutate(func(sa *corev1.Service) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (b *service) ObjectMeta(nf func(ObjectMeta)) *service {
	return b.Mutate(func(sa *corev1.Service) {
		omf := objectMeta(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Build()
	})
}

func (b *service) AddSelectorLabel(key, value string) *service {
	return b.Mutate(func(service *corev1.Service) {
		if service.Spec.Selector == nil {
			service.Spec.Selector = map[string]string{}
		}
		service.Spec.Selector[key] = value
	})
}

func (b *service) Ports(ports ...corev1.ServicePort) *service {
	return b.Mutate(func(service *corev1.Service) {
		service.Spec.Ports = ports
	})
}
