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

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/projectriff/system/pkg/apis"
	buildv1alpha1 "github.com/projectriff/system/pkg/apis/build/v1alpha1"
	rtesting "github.com/projectriff/system/pkg/controllers/testing"
	"github.com/projectriff/system/pkg/refs"
)

type function struct {
	target *buildv1alpha1.Function
}

func Function(seed ...*buildv1alpha1.Function) *function {
	var target *buildv1alpha1.Function
	switch len(seed) {
	case 0:
		target = &buildv1alpha1.Function{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &function{
		target: target,
	}
}

func (b *function) deepCopy() *function {
	return Function(b.target.DeepCopy())
}

func (b *function) Build() *buildv1alpha1.Function {
	return b.deepCopy().target
}

func (b *function) Mutate(m func(*buildv1alpha1.Function)) *function {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *function) NamespaceName(namespace, name string) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.ObjectMeta.Namespace = namespace
		fn.ObjectMeta.Name = name
	})
}

func (b *function) ObjectMeta(nf func(ObjectMeta)) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		omf := objectMeta(fn.ObjectMeta)
		nf(omf)
		fn.ObjectMeta = omf.Build()
	})
}

func (b *function) Image(format string, a ...interface{}) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Spec.Image = fmt.Sprintf(format, a...)
	})
}

func (b *function) Artifact(artifact string) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Spec.Artifact = artifact
	})
}

func (b *function) Handler(handler string) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Spec.Handler = handler
	})
}

func (b *function) Invoker(invoker string) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Spec.Invoker = invoker
	})
}

func (b *function) SourceGit(url string, revision string) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		if fn.Spec.Source == nil {
			fn.Spec.Source = &buildv1alpha1.Source{}
		}
		fn.Spec.Source = &buildv1alpha1.Source{
			Git: &buildv1alpha1.Git{
				URL:      url,
				Revision: revision,
			},
			SubPath: fn.Spec.Source.SubPath,
		}
	})
}

func (b *function) SourceSubPath(subpath string) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		if fn.Spec.Source == nil {
			fn.Spec.Source = &buildv1alpha1.Source{}
		}
		fn.Spec.Source.SubPath = subpath
	})
}

func (b *function) BuildCache(quantity string) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		size, err := resource.ParseQuantity(quantity)
		if err != nil {
			panic(err)
		}
		fn.Spec.CacheSize = &size
	})
}

func (b *function) StatusConditions(conditions ...*condition) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Build()
		}
		fn.Status.Conditions = c
	})
}

func (b *function) StatusReady() *function {
	return b.StatusConditions(
		Condition().Type(buildv1alpha1.FunctionConditionReady).True(),
	)
}

func (b *function) StatusObservedGeneration(generation int64) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Status.ObservedGeneration = generation
	})
}

func (b *function) StatusKpackImageRef(format string, a ...interface{}) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Status.KpackImageRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("build.pivotal.io"),
			Kind:     "Image",
			Name:     fmt.Sprintf(format, a...),
		}
	})
}

func (b *function) StatusBuildCacheRef(format string, a ...interface{}) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Status.BuildCacheRef = &refs.TypedLocalObjectReference{
			Kind: "PersistentVolumeClaim",
			Name: fmt.Sprintf(format, a...),
		}
	})
}

func (b *function) StatusTargetImage(format string, a ...interface{}) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Status.TargetImage = fmt.Sprintf(format, a...)
	})
}

func (b *function) StatusLatestImage(format string, a ...interface{}) *function {
	return b.Mutate(func(fn *buildv1alpha1.Function) {
		fn.Status.LatestImage = fmt.Sprintf(format, a...)
	})
}
