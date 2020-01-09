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
	corev1alpha1 "github.com/projectriff/system/pkg/apis/core/v1alpha1"
	rtesting "github.com/projectriff/system/pkg/controllers/testing"
	"github.com/projectriff/system/pkg/refs"
)

type deployerCore struct {
	target *corev1alpha1.Deployer
}

func DeployerCore(seed ...*corev1alpha1.Deployer) *deployerCore {
	var target *corev1alpha1.Deployer
	switch len(seed) {
	case 0:
		target = &corev1alpha1.Deployer{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &deployerCore{
		target: target,
	}
}

func (b *deployerCore) deepCopy() *deployerCore {
	return DeployerCore(b.target.DeepCopy())
}

func (b *deployerCore) Build() *corev1alpha1.Deployer {
	return b.deepCopy().target
}

func (b *deployerCore) Mutate(m func(*corev1alpha1.Deployer)) *deployerCore {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *deployerCore) NamespaceName(namespace, name string) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.ObjectMeta.Namespace = namespace
		deployer.ObjectMeta.Name = name
	})
}

func (b *deployerCore) ObjectMeta(nf func(ObjectMeta)) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		omf := objectMeta(deployer.ObjectMeta)
		nf(omf)
		deployer.ObjectMeta = omf.Build()
	})
}

func (b *deployerCore) PodTemplateSpec(nf func(PodTemplateSpec)) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		if deployer.Spec.Template == nil {
			deployer.Spec.Template = &corev1.PodTemplateSpec{}
		}
		ptsf := podTemplateSpec(*deployer.Spec.Template)
		nf(ptsf)
		template := ptsf.Build()
		deployer.Spec.Template = &template
	})
}

func (b *deployerCore) HandlerContainer(cb func(*corev1.Container)) *deployerCore {
	return b.PodTemplateSpec(func(pts PodTemplateSpec) {
		pts.ContainerNamed("handler", cb)
	})
}

func (b *deployerCore) ApplicationRef(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Spec.Build = &corev1alpha1.Build{
			ApplicationRef: fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerCore) ContainerRef(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Spec.Build = &corev1alpha1.Build{
			ContainerRef: fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerCore) FunctionRef(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Spec.Build = &corev1alpha1.Build{
			FunctionRef: fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerCore) Image(format string, a ...interface{}) *deployerCore {
	return b.HandlerContainer(func(container *corev1.Container) {
		container.Image = fmt.Sprintf(format, a...)
	})
}

func (b *deployerCore) IngressPolicy(policy corev1alpha1.IngressPolicy) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Spec.IngressPolicy = policy
	})
}

func (b *deployerCore) StatusConditions(conditions ...*condition) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Build()
		}
		deployer.Status.Conditions = c
	})
}

func (b *deployerCore) StatusObservedGeneration(generation int64) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Status.ObservedGeneration = generation
	})
}

func (b *deployerCore) StatusLatestImage(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Status.LatestImage = fmt.Sprintf(format, a...)
	})
}

func (b *deployerCore) StatusDeploymentRef(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Status.DeploymentRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("apps"),
			Kind:     "Deployment",
			Name:     fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerCore) StatusServiceRef(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Status.ServiceRef = &refs.TypedLocalObjectReference{
			APIGroup: nil,
			Kind:     "Service",
			Name:     fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerCore) StatusIngressRef(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Status.IngressRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("networking.k8s.io"),
			Kind:     "Ingress",
			Name:     fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerCore) StatusAddressURL(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Status.Address = &apis.Addressable{
			URL: fmt.Sprintf(format, a...),
		}
	})
}

func (b *deployerCore) StatusURL(format string, a ...interface{}) *deployerCore {
	return b.Mutate(func(deployer *corev1alpha1.Deployer) {
		deployer.Status.URL = fmt.Sprintf(format, a...)
	})
}
