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

	"github.com/projectriff/system/pkg/apis"
	knativeservingv1 "github.com/projectriff/system/pkg/apis/thirdparty/knative/serving/v1"
)

type knativeConfiguration struct {
	target *knativeservingv1.Configuration
}

func KnativeConfiguration(seed ...*knativeservingv1.Configuration) *knativeConfiguration {
	var target *knativeservingv1.Configuration
	switch len(seed) {
	case 0:
		target = &knativeservingv1.Configuration{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &knativeConfiguration{
		target: target,
	}
}

func (b *knativeConfiguration) deepCopy() *knativeConfiguration {
	return KnativeConfiguration(b.target.DeepCopy())
}

func (b *knativeConfiguration) Build() *knativeservingv1.Configuration {
	return b.deepCopy().target
}

func (b *knativeConfiguration) Mutate(m func(*knativeservingv1.Configuration)) *knativeConfiguration {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *knativeConfiguration) NamespaceName(namespace, name string) *knativeConfiguration {
	return b.Mutate(func(configuration *knativeservingv1.Configuration) {
		configuration.ObjectMeta.Namespace = namespace
		configuration.ObjectMeta.Name = name
	})
}

func (b *knativeConfiguration) ObjectMeta(nf func(ObjectMeta)) *knativeConfiguration {
	return b.Mutate(func(configuration *knativeservingv1.Configuration) {
		omf := objectMeta(configuration.ObjectMeta)
		nf(omf)
		configuration.ObjectMeta = omf.Build()
	})
}

func (b *knativeConfiguration) PodTemplateSpec(nf func(PodTemplateSpec)) *knativeConfiguration {
	return b.Mutate(func(configuration *knativeservingv1.Configuration) {
		ptsf := podTemplateSpec(
			// convert RevisionTemplateSpec into PodTemplateSpec
			corev1.PodTemplateSpec{
				ObjectMeta: configuration.Spec.Template.ObjectMeta,
				Spec:       configuration.Spec.Template.Spec.PodSpec,
			},
		)
		nf(ptsf)
		template := ptsf.Build()
		// update RevisionTemplateSpec with PodTemplateSpec managed fields
		configuration.Spec.Template.ObjectMeta = template.ObjectMeta
		configuration.Spec.Template.Spec.PodSpec = template.Spec
	})
}

func (b *knativeConfiguration) UserContainer(cb func(*corev1.Container)) *knativeConfiguration {
	return b.PodTemplateSpec(func(pts PodTemplateSpec) {
		pts.ContainerNamed("user-container", cb)
	})
}

func (b *knativeConfiguration) StatusConditions(conditions ...*condition) *knativeConfiguration {
	return b.Mutate(func(configuration *knativeservingv1.Configuration) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Build()
		}
		configuration.Status.Conditions = c
	})
}

func (b *knativeConfiguration) StatusReady() *knativeConfiguration {
	return b.StatusConditions(
		Condition().Type(knativeservingv1.ConfigurationConditionReady).True(),
	)
}

func (b *knativeConfiguration) StatusObservedGeneration(generation int64) *knativeConfiguration {
	return b.Mutate(func(configuration *knativeservingv1.Configuration) {
		configuration.Status.ObservedGeneration = generation
	})
}
