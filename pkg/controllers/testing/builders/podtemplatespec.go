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
	corev1 "k8s.io/api/core/v1"
)

type PodTemplateSpec interface {
	Mutate(m func(*corev1.PodTemplateSpec)) PodTemplateSpec
	Build() corev1.PodTemplateSpec

	AddLabel(key, value string) PodTemplateSpec
	AddAnnotation(key, value string) PodTemplateSpec
	ContainerNamed(name string, cb func(*corev1.Container)) PodTemplateSpec
}

type podTemplateSpecImpl struct {
	target *corev1.PodTemplateSpec
}

func podTemplateSpec(seed corev1.PodTemplateSpec) *podTemplateSpecImpl {
	return &podTemplateSpecImpl{
		target: &seed,
	}
}

func (b *podTemplateSpecImpl) Build() corev1.PodTemplateSpec {
	return *(b.target.DeepCopy())
}

func (b *podTemplateSpecImpl) Mutate(m func(*corev1.PodTemplateSpec)) PodTemplateSpec {
	m(b.target)
	return b
}

func (b *podTemplateSpecImpl) AddLabel(key, value string) PodTemplateSpec {
	return b.Mutate(func(pts *corev1.PodTemplateSpec) {
		if pts.Labels == nil {
			pts.Labels = map[string]string{}
		}
		pts.Labels[key] = value
	})
}

func (b *podTemplateSpecImpl) AddAnnotation(key, value string) PodTemplateSpec {
	return b.Mutate(func(pts *corev1.PodTemplateSpec) {
		if pts.Annotations == nil {
			pts.Annotations = map[string]string{}
		}
		pts.Annotations[key] = value
	})
}

func (b *podTemplateSpecImpl) ContainerNamed(name string, cb func(*corev1.Container)) PodTemplateSpec {
	return b.Mutate(func(pts *corev1.PodTemplateSpec) {
		found := false
		// check for existing container
		for i, container := range pts.Spec.Containers {
			if container.Name == name {
				found = true
				if cb != nil {
					// container mutations
					cb(&container)
					pts.Spec.Containers[i] = container
				}
				break
			}
		}
		if !found {
			// not found, create new container
			container := corev1.Container{Name: name}
			if cb != nil {
				// container mutations
				cb(&container)
			}
			pts.Spec.Containers = append(pts.Spec.Containers, container)
		}
	})
}
