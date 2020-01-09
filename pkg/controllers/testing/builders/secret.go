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
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type secret struct {
	target *corev1.Secret
}

func Secret(seed ...*corev1.Secret) *secret {
	var target *corev1.Secret
	switch len(seed) {
	case 0:
		target = &corev1.Secret{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &secret{
		target: target,
	}
}

func (b *secret) deepCopy() *secret {
	return Secret(b.target.DeepCopy())
}

func (b *secret) Build() *corev1.Secret {
	return b.deepCopy().target
}

func (b *secret) Mutate(m func(*corev1.Secret)) *secret {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *secret) NamespaceName(namespace, name string) *secret {
	return b.Mutate(func(s *corev1.Secret) {
		s.ObjectMeta.Namespace = namespace
		s.ObjectMeta.Name = name
	})
}

func (b *secret) ObjectMeta(nf func(ObjectMeta)) *secret {
	return b.Mutate(func(s *corev1.Secret) {
		omf := objectMeta(s.ObjectMeta)
		nf(omf)
		s.ObjectMeta = omf.Build()
	})
}

func (b *secret) Type(t corev1.SecretType) *secret {
	return b.Mutate(func(s *corev1.Secret) {
		s.Type = t
	})
}

func (b *secret) AddData(key, value string) *secret {
	return b.Mutate(func(s *corev1.Secret) {
		if s.Data == nil {
			s.Data = map[string][]byte{}
		}
		encoded := []byte{}
		base64.StdEncoding.Encode(encoded, []byte(value))
		s.Data[key] = encoded
	})
}
