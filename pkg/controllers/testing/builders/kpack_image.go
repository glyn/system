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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/projectriff/system/pkg/apis"
	kpackbuildv1alpha1 "github.com/projectriff/system/pkg/apis/thirdparty/kpack/build/v1alpha1"
)

type kpackImage struct {
	target *kpackbuildv1alpha1.Image
}

func KpackImage(seed ...*kpackbuildv1alpha1.Image) *kpackImage {
	var target *kpackbuildv1alpha1.Image
	switch len(seed) {
	case 0:
		target = &kpackbuildv1alpha1.Image{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &kpackImage{
		target: target,
	}
}

func (b *kpackImage) deepCopy() *kpackImage {
	return KpackImage(b.target.DeepCopy())
}

func (b *kpackImage) Build() *kpackbuildv1alpha1.Image {
	return b.deepCopy().target
}

func (b *kpackImage) Mutate(m func(*kpackbuildv1alpha1.Image)) *kpackImage {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *kpackImage) NamespaceName(namespace, name string) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.ObjectMeta.Namespace = namespace
		image.ObjectMeta.Name = name
	})
}

func (b *kpackImage) ObjectMeta(nf func(ObjectMeta)) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		omf := objectMeta(image.ObjectMeta)
		nf(omf)
		image.ObjectMeta = omf.Build()
	})
}

func (b *kpackImage) ApplicationBuilder() *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Spec.Builder = kpackbuildv1alpha1.ImageBuilder{
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterBuilder",
			},
			Name: "riff-application",
		}
		image.Spec.ServiceAccount = "riff-build"
	})
}

func (b *kpackImage) FunctionBuilder(artifact, handler, invoker string) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Spec.Builder = kpackbuildv1alpha1.ImageBuilder{
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterBuilder",
			},
			Name: "riff-function",
		}
		image.Spec.ServiceAccount = "riff-build"
		env := []corev1.EnvVar{}
		for _, envvar := range image.Spec.Build.Env {
			// filter existing value
			if envvar.Name != "RIFF" && envvar.Name != "RIFF_ARTIFACT" && envvar.Name != "RIFF_HANDLER" && envvar.Name != "RIFF_OVERRIDE" {
				env = append(env, envvar)
			}
		}
		// add new values
		image.Spec.Build.Env = append(env,
			corev1.EnvVar{
				Name:  "RIFF",
				Value: "true",
			},
			corev1.EnvVar{
				Name:  "RIFF_ARTIFACT",
				Value: artifact,
			},
			corev1.EnvVar{
				Name:  "RIFF_HANDLER",
				Value: handler,
			},
			corev1.EnvVar{
				Name:  "RIFF_OVERRIDE",
				Value: invoker,
			},
		)
	})
}

func (b *kpackImage) Tag(format string, a ...interface{}) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Spec.Tag = fmt.Sprintf(format, a...)
	})
}

func (b *kpackImage) SourceGit(url string, revision string) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Spec.Source = kpackbuildv1alpha1.SourceConfig{
			Git: &kpackbuildv1alpha1.Git{
				URL:      url,
				Revision: revision,
			},
			SubPath: image.Spec.Source.SubPath,
		}
	})
}

func (b *kpackImage) SourceSubPath(subpath string) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Spec.Source.SubPath = subpath
	})
}

func (b *kpackImage) BuildCache(quantity string) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		size, err := resource.ParseQuantity(quantity)
		if err != nil {
			panic(err)
		}
		image.Spec.CacheSize = &size
	})
}

func (b *kpackImage) StatusConditions(conditions ...*condition) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Build()
		}
		image.Status.Conditions = c
	})
}

func (b *kpackImage) StatusReady() *kpackImage {
	return b.StatusConditions(
		Condition().Type(apis.ConditionReady).True(),
	)
}

func (b *kpackImage) StatusObservedGeneration(generation int64) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Status.ObservedGeneration = generation
	})
}

func (b *kpackImage) StatusLatestImage(format string, a ...interface{}) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Status.LatestImage = fmt.Sprintf(format, a...)
	})
}

func (b *kpackImage) StatusBuildCacheName(format string, a ...interface{}) *kpackImage {
	return b.Mutate(func(image *kpackbuildv1alpha1.Image) {
		image.Status.BuildCacheName = fmt.Sprintf(format, a...)
	})
}
