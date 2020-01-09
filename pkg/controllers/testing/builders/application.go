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

type application struct {
	target *buildv1alpha1.Application
}

func Application(seed ...*buildv1alpha1.Application) *application {
	var target *buildv1alpha1.Application
	switch len(seed) {
	case 0:
		target = &buildv1alpha1.Application{}
	case 1:
		target = seed[0]
	default:
		panic(fmt.Errorf("expected exactly zero or one seed, got %v", seed))
	}
	return &application{
		target: target,
	}
}

func (b *application) deepCopy() *application {
	return Application(b.target.DeepCopy())
}

func (b *application) Build() *buildv1alpha1.Application {
	return b.deepCopy().target
}

func (b *application) Mutate(m func(*buildv1alpha1.Application)) *application {
	b = b.deepCopy()
	m(b.target)
	return b
}

func (b *application) NamespaceName(namespace, name string) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		app.ObjectMeta.Namespace = namespace
		app.ObjectMeta.Name = name
	})
}

func (b *application) ObjectMeta(nf func(ObjectMeta)) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		omf := objectMeta(app.ObjectMeta)
		nf(omf)
		app.ObjectMeta = omf.Build()
	})
}

func (b *application) Image(format string, a ...interface{}) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		app.Spec.Image = fmt.Sprintf(format, a...)
	})
}

func (b *application) SourceGit(url string, revision string) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		if app.Spec.Source == nil {
			app.Spec.Source = &buildv1alpha1.Source{}
		}
		app.Spec.Source = &buildv1alpha1.Source{
			Git: &buildv1alpha1.Git{
				URL:      url,
				Revision: revision,
			},
			SubPath: app.Spec.Source.SubPath,
		}
	})
}

func (b *application) SourceSubPath(subpath string) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		if app.Spec.Source == nil {
			app.Spec.Source = &buildv1alpha1.Source{}
		}
		app.Spec.Source.SubPath = subpath
	})
}

func (b *application) BuildCache(quantity string) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		size, err := resource.ParseQuantity(quantity)
		if err != nil {
			panic(err)
		}
		app.Spec.CacheSize = &size
	})
}

func (b *application) StatusConditions(conditions ...*condition) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		c := make([]apis.Condition, len(conditions))
		for i, cg := range conditions {
			c[i] = cg.Build()
		}
		app.Status.Conditions = c
	})
}

func (b *application) StatusReady() *application {
	return b.StatusConditions(
		Condition().Type(buildv1alpha1.ApplicationConditionReady).True(),
	)
}

func (b *application) StatusObservedGeneration(generation int64) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		app.Status.ObservedGeneration = generation
	})
}

func (b *application) StatusKpackImageRef(format string, a ...interface{}) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		app.Status.KpackImageRef = &refs.TypedLocalObjectReference{
			APIGroup: rtesting.StringPtr("build.pivotal.io"),
			Kind:     "Image",
			Name:     fmt.Sprintf(format, a...),
		}
	})
}

func (b *application) StatusBuildCacheRef(format string, a ...interface{}) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		app.Status.BuildCacheRef = &refs.TypedLocalObjectReference{
			Kind: "PersistentVolumeClaim",
			Name: fmt.Sprintf(format, a...),
		}
	})
}

func (b *application) StatusTargetImage(format string, a ...interface{}) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		app.Status.TargetImage = fmt.Sprintf(format, a...)
	})
}

func (b *application) StatusLatestImage(format string, a ...interface{}) *application {
	return b.Mutate(func(app *buildv1alpha1.Application) {
		app.Status.LatestImage = fmt.Sprintf(format, a...)
	})
}
