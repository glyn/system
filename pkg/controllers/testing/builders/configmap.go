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

type configMap struct {
	target *corev1.ConfigMap
}

func ConfigMap(seed ...*corev1.ConfigMap) *configMap {
	var target *corev1.ConfigMap
	switch len(seed) {
	case 0:
		target = &corev1.ConfigMap{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &configMap{
		target: target,
	}
}

func (b *configMap) deepCopy() *configMap {
	return ConfigMap(b.target.DeepCopy())
}

func (b *configMap) Build() *corev1.ConfigMap {
	return b.deepCopy().target
}

func (b *configMap) Mutate(m func(*corev1.ConfigMap)) *configMap {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *configMap) NamespaceName(namespace, name string) *configMap {
	return b.Mutate(func(cm *corev1.ConfigMap) {
		cm.ObjectMeta.Namespace = namespace
		cm.ObjectMeta.Name = name
	})
}

func (b *configMap) ObjectMeta(nf func(ObjectMeta)) *configMap {
	return b.Mutate(func(cm *corev1.ConfigMap) {
		omf := objectMeta(cm.ObjectMeta)
		nf(omf)
		cm.ObjectMeta = omf.Build()
	})
}

func (b *configMap) AddData(key, value string) *configMap {
	return b.Mutate(func(cm *corev1.ConfigMap) {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[key] = value
	})
}
