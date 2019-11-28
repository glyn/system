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

package streaming_test

import (
	"context"
	"sync"
	"time"

	"github.com/projectriff/system/pkg/apis"
	"github.com/projectriff/system/pkg/tracker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	streamingv1alpha1 "github.com/projectriff/system/pkg/apis/streaming/v1alpha1"
	"github.com/projectriff/system/pkg/controllers/streaming"
)

const (
	waitTime        = "60s"
	pollingInterval = "100ms"
)

// ultimately is Eventually at "kubernetes speed"
func ultimately(actual interface{}) AsyncAssertion {
	return Eventually(actual, waitTime, pollingInterval)
}

var _ = Describe("Processor Controller Integration Test", func() {
	const (
		testNamespace  = "namespace"
		testProcessor  = "myprocessor"
		testGeneration = int64(1)
	)

	var (
		clnt       client.Client
		stopCh     chan struct{}
		stopMgr    chan struct{}
		mgrStopped *sync.WaitGroup
	)

	BeforeEach(func() {
		// Setup the Manager and Controller.
		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		clnt = mgr.GetClient()

		stopCh = make(chan struct{})

		processorLog := ctrl.Log.WithName("controllers").WithName("Processor")
		err = (&streaming.ProcessorReconciler{
			Client:    clnt,
			Log:       processorLog,
			Tracker:   tracker.New(time.Hour, processorLog.WithName("tracker")),
			Namespace: testNamespace,
		}).SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		stopMgr, mgrStopped = StartTestManager(mgr)
	})

	AfterEach(func() {
		close(stopCh)
		close(stopMgr)
		mgrStopped.Wait()
	})

	Context("when a processor is created", func() {
		var processor *streamingv1alpha1.Processor

		BeforeEach(func() {
			processor = &streamingv1alpha1.Processor{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:       testProcessor,
					Namespace:  testNamespace,
					Generation: testGeneration,
				},
				Spec: streamingv1alpha1.ProcessorSpec{
					Build: &streamingv1alpha1.Build{
						ContainerRef: "testContainer",
						FunctionRef:  "testFunction",
					},
				},
				Status: streamingv1alpha1.ProcessorStatus{},
			}
			processor.Default()
			createProcessor(clnt, processor)
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "riff-streaming-processor",
					Namespace: testNamespace,
				},
				Data: map[string]string{"processorImage": "projectriff/streaming-processor:0.0"},
			}
			Expect(clnt.Create(context.TODO(), cm)).To(Succeed())
		})

		AfterEach(func() {
			safelyDeleteProcessor(clnt, processor)
		})

		Context("when the processor's deployment becomes ready", func() {
			BeforeEach(func() {
				// TODO: set deployment status Ready condition to true
				var deployment *appsv1.Deployment
				ultimately(func() error {
					var deployments appsv1.DeploymentList
					if err := clnt.List(context.TODO(), &deployments, client.MatchingLabels(map[string]string{})); err != nil {
						return err
					}
					Expect(len(deployments.Items)).To(Equal(1))
					deployment = &deployments.Items[0]
					return nil
				}).Should(Succeed())
			})

			FIt("should become ready", func() {
				Eventually(func() corev1.ConditionStatus { // FIXME: replace with ultimately before submitting PR
					var proc streamingv1alpha1.Processor
					if err := clnt.Get(context.TODO(), client.ObjectKey{Namespace: testNamespace, Name: testProcessor}, &proc); err == nil {
						for _, cond := range proc.Status.Conditions {
							if cond.Type == apis.ConditionReady {
								return cond.Status
							}
						}
					}
					return corev1.ConditionUnknown
				}, "10s").Should(Equal(corev1.ConditionTrue))
			})
		})
	})
})

func createProcessor(clnt client.Client, proc *streamingv1alpha1.Processor) {
	// wait until the processor can be created as it may still be being deleted
	ultimately(func() error {
		err := clnt.Create(context.TODO(), proc)
		Expect(err).To(Or(Succeed(), MatchError(ContainSubstring("object is being deleted"))))
		return err
	}).Should(Succeed())

}

func safelyDeleteProcessor(clnt client.Client, processor *streamingv1alpha1.Processor) {
	_ = clnt.Delete(context.TODO(), processor) // ignore any error in case the test has deleted the processor
}
