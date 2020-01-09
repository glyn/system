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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rtesting "github.com/projectriff/system/pkg/controllers/testing"
)

type deployment struct {
	target *appsv1.Deployment
}

func Deployment(seed ...*appsv1.Deployment) *deployment {
	var target *appsv1.Deployment
	switch len(seed) {
	case 0:
		target = &appsv1.Deployment{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &deployment{
		target: target,
	}
}

func (b *deployment) deepCopy() *deployment {
	return Deployment(b.target.DeepCopy())
}

func (b *deployment) Build() *appsv1.Deployment {
	return b.deepCopy().target
}

func (b *deployment) Mutate(m func(*appsv1.Deployment)) *deployment {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *deployment) NamespaceName(namespace, name string) *deployment {
	return b.Mutate(func(sa *appsv1.Deployment) {
		sa.ObjectMeta.Namespace = namespace
		sa.ObjectMeta.Name = name
	})
}

func (b *deployment) ObjectMeta(nf func(ObjectMeta)) *deployment {
	return b.Mutate(func(sa *appsv1.Deployment) {
		omf := objectMeta(sa.ObjectMeta)
		nf(omf)
		sa.ObjectMeta = omf.Build()
	})
}

func (b *deployment) PodTemplateSpec(nf func(PodTemplateSpec)) *deployment {
	return b.Mutate(func(deployment *appsv1.Deployment) {
		ptsf := podTemplateSpec(deployment.Spec.Template)
		nf(ptsf)
		deployment.Spec.Template = ptsf.Build()
	})
}

func (b *deployment) HandlerContainer(cb func(*corev1.Container)) *deployment {
	return b.PodTemplateSpec(func(pts PodTemplateSpec) {
		pts.ContainerNamed("handler", cb)
	})
}

func (b *deployment) Replicas(replicas int32) *deployment {
	return b.Mutate(func(deployment *appsv1.Deployment) {
		deployment.Spec.Replicas = rtesting.Int32Ptr(replicas)
	})
}

func (b *deployment) AddSelectorLabel(key, value string) *deployment {
	return b.Mutate(func(deployment *appsv1.Deployment) {
		if deployment.Spec.Selector == nil {
			deployment.Spec.Selector = &metav1.LabelSelector{}
		}
		metav1.AddLabelToSelector(deployment.Spec.Selector, key, value)
		deployment.Spec.Template = podTemplateSpec(deployment.Spec.Template).AddLabel(key, value).Build()
	})
}

func (b *deployment) StatusConditions(conditions ...*condition) *deployment {
	return b.Mutate(func(deployment *appsv1.Deployment) {
		c := make([]appsv1.DeploymentCondition, len(conditions))
		for i, cg := range conditions {
			dc := cg.Build()
			c[i] = appsv1.DeploymentCondition{
				Type:    appsv1.DeploymentConditionType(dc.Type),
				Status:  dc.Status,
				Reason:  dc.Reason,
				Message: dc.Message,
			}
		}
		deployment.Status.Conditions = c
	})
}
