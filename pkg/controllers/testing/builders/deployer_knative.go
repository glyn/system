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
	knativev1alpha1 "github.com/projectriff/system/pkg/apis/knative/v1alpha1"
	rtesting "github.com/projectriff/system/pkg/controllers/testing"
	"github.com/projectriff/system/pkg/refs"
)

type deployerKnative struct {
	target *knativev1alpha1.Deployer
}

func DeployerKnative(seed ...*knativev1alpha1.Deployer) *deployerKnative {
	var target *knativev1alpha1.Deployer
	switch len(seed) {
	case 0:
		target = &knativev1alpha1.Deployer{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &deployerKnative{
		target: target,
	}
}

func (b *deployerKnative) deepCopy() *deployerKnative {
	return DeployerKnative(b.target.DeepCopy())
}

func (b *deployerKnative) Build() *knativev1alpha1.Deployer {
	return b.deepCopy().target
}

func (b *deployerKnative) Mutate(m func(*knativev1alpha1.Deployer)) *deployerKnative {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *deployerKnative) NamespaceName(namespace, name string) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.ObjectMeta.Namespace = namespace
		deployer.ObjectMeta.Name = name
	})
}

func (b *deployerKnative) ObjectMeta(nf func(ObjectMeta)) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		omf := objectMeta(deployer.ObjectMeta)
		nf(omf)
		deployer.ObjectMeta = omf.Build()
	})
}

func (b *deployerKnative) PodTemplateSpec(nf func(PodTemplateSpec)) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		if deployer.Spec.Template == nil {
			deployer.Spec.Template = &corev1.PodTemplateSpec{}
		}
		ptsf := podTemplateSpec(*deployer.Spec.Template)
		nf(ptsf)
		pts := ptsf.Build()
		deployer.Spec.Template = &pts
	})
}

func (b *deployerKnative) ApplicationRef(format string, a ...interface{}) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Spec.Build = &knativev1alpha1.Build{
			ApplicationRef: fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerKnative) ContainerRef(format string, a ...interface{}) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Spec.Build = &knativev1alpha1.Build{
			ContainerRef: fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerKnative) FunctionRef(format string, a ...interface{}) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Spec.Build = &knativev1alpha1.Build{
			FunctionRef: fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerKnative) Image(format string, a ...interface{}) *deployerKnative {
	return b.PodTemplateSpec(func(ptsf PodTemplateSpec) {
		ptsf.ContainerNamed("user-container", func(container *corev1.Container) {
			container.Image = fmt.Sprintf(format, a...)
		})
	})
}

func (b *deployerKnative) IngressPolicy(policy knativev1alpha1.IngressPolicy) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Spec.IngressPolicy = policy
	})
}

func (b *deployerKnative) MinScale(scale int32) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Spec.Scale.Min = &scale
	})
}

func (b *deployerKnative) MaxScale(scale int32) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Spec.Scale.Max = &scale
	})
}

func (b *deployerKnative) StatusConditions(conditions ...*condition) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Build()
		}
		deployer.Status.Conditions = c
	})
}

func (b *deployerKnative) StatusObservedGeneration(generation int64) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Status.ObservedGeneration = generation
	})
}

func (b *deployerKnative) StatusLatestImage(format string, a ...interface{}) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Status.LatestImage = fmt.Sprintf(format, a...)
	})
}

func (b *deployerKnative) StatusConfigurationRef(format string, a ...interface{}) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Status.ConfigurationRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("serving.knative.dev"),
			Kind:     "Configuration",
			Name:     fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerKnative) StatusRouteRef(format string, a ...interface{}) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Status.RouteRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("serving.knative.dev"),
			Kind:     "Route",
			Name:     fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerKnative) StatusAddressURL(url string) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Status.Address = &apis.Addressable{
			URL: url,
		}
	})
}

func (b *deployerKnative) StatusURL(url string) *deployerKnative {
	return b.Mutate(func(deployer *knativev1alpha1.Deployer) {
		deployer.Status.URL = url
	})
}
