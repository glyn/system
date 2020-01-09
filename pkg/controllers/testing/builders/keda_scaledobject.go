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

	kedav1alpha1 "github.com/projectriff/system/pkg/apis/thirdparty/keda/v1alpha1"
)

type kedaScaledObject struct {
	target *kedav1alpha1.ScaledObject
}

func KedaScaledObject(seed ...*kedav1alpha1.ScaledObject) *kedaScaledObject {
	var target *kedav1alpha1.ScaledObject
	switch len(seed) {
	case 0:
		target = &kedav1alpha1.ScaledObject{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &kedaScaledObject{
		target: target,
	}
}

func (b *kedaScaledObject) deepCopy() *kedaScaledObject {
	return KedaScaledObject(b.target.DeepCopy())
}

func (b *kedaScaledObject) Build() *kedav1alpha1.ScaledObject {
	return b.deepCopy().target
}

func (b *kedaScaledObject) Mutate(m func(*kedav1alpha1.ScaledObject)) *kedaScaledObject {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *kedaScaledObject) NamespaceName(namespace, name string) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		s.ObjectMeta.Namespace = namespace
		s.ObjectMeta.Name = name
	})
}

func (b *kedaScaledObject) ObjectMeta(nf func(ObjectMeta)) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		omf := objectMeta(s.ObjectMeta)
		nf(omf)
		s.ObjectMeta = omf.Build()
	})
}

func (b *kedaScaledObject) Spec(spec *kedav1alpha1.ScaledObjectSpec) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		s.Spec = *spec
	})
}

func (b *kedaScaledObject) ScaleTargetRefDeployment(format string, a ...interface{}) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		s.Spec.ScaleTargetRef = &kedav1alpha1.ObjectReference{DeploymentName: fmt.Sprintf(format, a...)}
	})
}

func (b *kedaScaledObject) PollingInterval(pollingInterval int32) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		s.Spec.PollingInterval = &pollingInterval
	})
}

func (b *kedaScaledObject) CooldownPeriod(cooldownPeriod int32) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		s.Spec.CooldownPeriod = &cooldownPeriod
	})
}

func (b *kedaScaledObject) MinReplicaCount(minReplicaCount int32) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		s.Spec.MinReplicaCount = &minReplicaCount
	})
}

func (b *kedaScaledObject) MaxReplicaCount(maxReplicaCount int32) *kedaScaledObject {
	return b.Mutate(func(s *kedav1alpha1.ScaledObject) {
		s.Spec.MaxReplicaCount = &maxReplicaCount
	})
}
