/*
Copyright 2020 the original author or authors.

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
	streamingv1alpha1 "github.com/projectriff/system/pkg/apis/streaming/v1alpha1"
	rtesting "github.com/projectriff/system/pkg/controllers/testing"
	"github.com/projectriff/system/pkg/refs"
)

type processor struct {
	target *streamingv1alpha1.Processor
}

func Processor(seed ...*streamingv1alpha1.Processor) *processor {
	var target *streamingv1alpha1.Processor
	switch len(seed) {
	case 0:
		target = &streamingv1alpha1.Processor{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &processor{
		target: target,
	}
}

func (b *processor) deepCopy() *processor {
	return Processor(b.target.DeepCopy())
}

func (b *processor) Build() *streamingv1alpha1.Processor {
	return b.deepCopy().target
}

func (b *processor) Mutate(m func(*streamingv1alpha1.Processor)) *processor {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *processor) NamespaceName(namespace, name string) *processor {
	return b.Mutate(func(p *streamingv1alpha1.Processor) {
		p.ObjectMeta.Namespace = namespace
		p.ObjectMeta.Name = name
	})
}

func (b *processor) ObjectMeta(nf func(ObjectMeta)) *processor {
	return b.Mutate(func(s *streamingv1alpha1.Processor) {
		omf := objectMeta(s.ObjectMeta)
		nf(omf)
		s.ObjectMeta = omf.Build()
	})
}

func (b *processor) PodTemplateSpec(nf func(PodTemplateSpec)) *processor {
	return b.Mutate(func(processor *streamingv1alpha1.Processor) {
		var ptsf *podTemplateSpecImpl
		if processor.Spec.Template != nil {
			ptsf = podTemplateSpec(*processor.Spec.Template)
		} else {
			ptsf = podTemplateSpec(corev1.PodTemplateSpec{})
		}
		nf(ptsf)
		templateSpec := ptsf.Build()
		processor.Spec.Template = &templateSpec
	})
}

func (b *processor) StatusConditions(conditions ...*condition) *processor {
	return b.Mutate(func(processor *streamingv1alpha1.Processor) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			dc := cg.Build()
			c[i] = apis.Condition{
				Type:    apis.ConditionType(dc.Type),
				Status:  dc.Status,
				Reason:  dc.Reason,
				Message: dc.Message,
			}
		}
		processor.Status.Conditions = c
	})
}

func (b *processor) StatusLatestImage(image string) *processor {
	return b.Mutate(func(proc *streamingv1alpha1.Processor) {
		proc.Status.LatestImage = image
	})
}

func (b *processor) StatusDeploymentRef(deploymentName string) *processor {
	return b.Mutate(func(proc *streamingv1alpha1.Processor) {
		proc.Status.DeploymentRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("apps"),
			Kind:     "Deployment",
			Name:     deploymentName,
		}
	})
}

func (b *processor) SpecBuildFunctionRef(functionName string) *processor {
	return b.Mutate(func(proc *streamingv1alpha1.Processor) {
		proc.Spec.Build = &streamingv1alpha1.Build{
			FunctionRef: functionName,
		}
	})
}

func (b *processor) SpecBuildContainerRef(containerName string) *processor {
	return b.Mutate(func(proc *streamingv1alpha1.Processor) {
		proc.Spec.Build = &streamingv1alpha1.Build{
			ContainerRef: containerName,
		}
	})
}

func (b *processor) StatusScaledObjectRef(deploymentName string) *processor {
	return b.Mutate(func(proc *streamingv1alpha1.Processor) {
		proc.Status.ScaledObjectRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("keda.k8s.io"),
			Kind:     "ScaledObject",
			Name:     deploymentName,
		}
	})
}
