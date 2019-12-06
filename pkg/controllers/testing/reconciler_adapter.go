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

package testing

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gotesting "testing"
)

// Adapt wraps a controller runtime Reconciler to be a Knative controller Reconciler for the purpose of testing
func Adapt(rec reconcile.Reconciler, t *gotesting.T) controller.Reconciler {
	return &adapter{rec, t}
}

type adapter struct {
	controllerRuntimeRec reconcile.Reconciler
	t                    *gotesting.T
}

func (ad adapter) Reconcile(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	ad.t.Error(err)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
	_, err = ad.controllerRuntimeRec.Reconcile(req) // FIXME: capture the returned result somewhere so it can be tested
	return err
}

