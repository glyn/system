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

package streaming

import (
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	buildv1alpha1 "github.com/projectriff/system/pkg/apis/build/v1alpha1"
	streamingv1alpha1 "github.com/projectriff/system/pkg/apis/streaming/v1alpha1"
	kedav1alpha1 "github.com/projectriff/system/pkg/apis/thirdparty/keda/v1alpha1"
	rtesting "github.com/projectriff/system/pkg/controllers/testing"
	"github.com/projectriff/system/pkg/controllers/testing/builders"
	"github.com/projectriff/system/pkg/tracker"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = streamingv1alpha1.AddToScheme(scheme)
	_ = kedav1alpha1.AddToScheme(scheme)
	_ = buildv1alpha1.AddToScheme(scheme)

	const (
		testSha256         = "faa5faa5faa5faa5faa5faa5faa5faa5faa5faa5faa5faa5faa5faa5faa5faa5"
		testNamespace      = "test-namespace"
		testName           = "test-processor"
		testProcessorImage = "test-processor-image@sha256:" + testSha256
		testDefaultImage   = "test-default-image@sha256:" + testSha256
		testFunction       = "test-function"
		testFunctionImage  = "test-function-image@sha256:" + testSha256
		testContainer      = "test-container"
		testContainerImage = "test-container-image@sha256:" + testSha256
	)

	processorGiven := builders.Processor().
		NamespaceName(testNamespace, testName).
		PodTemplateSpec(func(spec builders.PodTemplateSpec) {
			spec.ContainerNamed(testContainer, func(c *corev1.Container) {
				c.Image = testDefaultImage
			})
		})
	imageNamesConfigMapGiven := builders.ConfigMap().
		NamespaceName(testNamespace, processorImages).
		AddData(processorImageKey, testProcessorImage)
	containerGiven := builders.Container().
		NamespaceName(testNamespace, testContainer).
		StatusLatestImage(testContainerImage)
	functionGiven := builders.Function().
		NamespaceName(testNamespace, testFunction)
	deploymentCreate := builders.Deployment().
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Namespace(testNamespace)
			om.GenerateName("%s-processor-", testName)
			om.AddLabel("streaming.projectriff.io/processor", testName)
			om.ControlledBy(processorGiven.Build(), scheme)
		}).
		Replicas(1).
		AddSelectorLabel("streaming.projectriff.io/processor", testName).
		PodTemplateSpec(func(pts builders.PodTemplateSpec) {
			pts.AddLabel("streaming.projectriff.io/processor", testName)
		})
	scaledObjectCreate := builders.KedaScaledObject().
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Namespace(testNamespace)
			om.GenerateName("%s-processor-", testName)
			om.AddLabel("streaming.projectriff.io/processor", testName)
			om.AddLabel("deploymentName", testName+"-processor-001") // TODO: this label looks bogus
			om.ControlledBy(processorGiven.Build(), scheme)
		}).
		ScaleTargetRefDeployment("%s-processor-001", testName).
		PollingInterval(1).
		CooldownPeriod(30).
		MinReplicaCount(1).
		MaxReplicaCount(30)

	testCoreContainer := func(imageRef string) func(container *corev1.Container) {
		return func(container *corev1.Container) {
			container.Name = testContainer
			container.Image = imageRef
			container.Ports = []corev1.ContainerPort{{ContainerPort: 8081}}
		}
	}

	processorCoreContainer := func(container *corev1.Container) {
		container.Name = "processor"
		container.Image = testProcessorImage
		container.Env = []corev1.EnvVar{
			{Name: "CNB_BINDINGS", Value: "/var/riff/bindings"},
			{Name: "INPUT_START_OFFSETS"},
			{Name: "INPUT_NAMES"},
			{Name: "OUTPUT_NAMES"},
			{Name: "GROUP", Value: testName},
			{Name: "FUNCTION", Value: "localhost:8081"},
		}
		container.ImagePullPolicy = "IfNotPresent"
	}

	processorConditionDeploymentReady := builders.Condition().Type(streamingv1alpha1.ProcessorConditionDeploymentReady)
	processorConditionReady := builders.Condition().Type(streamingv1alpha1.ProcessorConditionReady)
	processorConditionScaledObjectReady := builders.Condition().Type(streamingv1alpha1.ProcessorConditionScaledObjectReady)
	processorConditionStreamsReady := builders.Condition().Type(streamingv1alpha1.ProcessorConditionStreamsReady)

	table := rtesting.Table{{
		Name:         "processor does not exist",
		Key:          types.NamespacedName{Namespace: testNamespace, Name: testName},
		ExpectTracks: []rtesting.TrackRequest{},
	}, {
		Name: "getting processor fails",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Processor"),
		},
		ExpectTracks: []rtesting.TrackRequest{},
		ShouldErr:    true,
	}, {
		Name: "configMap does not exist",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.Build(),
		},
		ShouldErr: true,
		Verify:    rtesting.AssertErrorMessagef("configmaps %q not found", processorImages),
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "processor is marked for deletion",
		GivenObjects: []runtime.Object{
			processorGiven.
				ObjectMeta(func(meta builders.ObjectMeta) {
					meta.Deleted(1)
				}).
				Build(),
		},
		Key: types.NamespacedName{Namespace: testNamespace, Name: testName},
	}, {
		Name: "getting configMap fails",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.Build(),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "ConfigMap"),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
		},
		ShouldErr: true,
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "processor sidecar image not present in configMap",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.Build(),
			builders.ConfigMap().
				NamespaceName(testNamespace, processorImages).
				Build(),
		},
		ShouldErr: true,
		Verify:    rtesting.AssertErrorMessagef("missing processor image configuration"),
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.Unknown(),
				).
				StatusLatestImage(testDefaultImage).
				Build(),
		},
	}, {
		Name: "default application image not set",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			builders.Processor().
				NamespaceName(testNamespace, testName).
				Build(),
			imageNamesConfigMapGiven.Build(),
		},
		ShouldErr: true,
		Verify:    rtesting.AssertErrorMessagef("could not resolve an image"),
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "successful reconciliation",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.Build(),
			imageNamesConfigMapGiven.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.ContainerNamed(testContainer, testCoreContainer(testDefaultImage))
					pts.ContainerNamed("processor", processorCoreContainer)
				}).
				Build(),
			scaledObjectCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.True(),
					processorConditionStreamsReady.True(),
				).
				StatusLatestImage(testDefaultImage).
				StatusDeploymentRef(testName + "-processor-001").
				StatusScaledObjectRef(testName + "-processor-002").
				Build(),
		},
	}, {
		Name: "deployment creation fails",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.Build(),
			imageNamesConfigMapGiven.Build(),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Deployment"),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.ContainerNamed(testContainer, testCoreContainer(testDefaultImage))
					pts.ContainerNamed("processor", processorCoreContainer)
				}).
				Build(),
		},
		ShouldErr: true,
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.Unknown(),
				).
				StatusLatestImage(testDefaultImage).
				Build(),
		},
	}, {
		Name: "scaled object creation fails",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.Build(),
			imageNamesConfigMapGiven.Build(),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "ScaledObject"),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.ContainerNamed(testContainer, testCoreContainer(testDefaultImage))
					pts.ContainerNamed("processor", processorCoreContainer)
				}).
				Build(),
			scaledObjectCreate.Build(),
		},
		ShouldErr: true,
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.True(),
				).
				StatusLatestImage(testDefaultImage).
				StatusDeploymentRef(testName + "-processor-001").
				Build(),
		},
	}, {
		Name: "successful reconciliation with unsatisfied function reference",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.
				SpecBuildFunctionRef(testFunction).
				Build(),
			imageNamesConfigMapGiven.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
			rtesting.NewTrackRequest(functionGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "get function fails",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.
				SpecBuildFunctionRef(testFunction).
				Build(),
			imageNamesConfigMapGiven.Build(),
			functionGiven.Build(),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Function"),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
			rtesting.NewTrackRequest(functionGiven.Build(), processorGiven.Build(), scheme),
		},
		ShouldErr: true,
	}, {
		Name: "successful reconciliation with satisfied function reference",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.
				SpecBuildFunctionRef(testFunction).
				Build(),
			imageNamesConfigMapGiven.Build(),
			functionGiven.
				StatusLatestImage(testFunctionImage).
				Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
			rtesting.NewTrackRequest(functionGiven.Build(), processorGiven.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.ContainerNamed(testContainer, testCoreContainer(testFunctionImage))
					pts.ContainerNamed("processor", processorCoreContainer)
				}).
				Build(),
			scaledObjectCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.True(),
					processorConditionStreamsReady.True(),
				).
				StatusLatestImage(testFunctionImage).
				StatusDeploymentRef(testName + "-processor-001").
				StatusScaledObjectRef(testName + "-processor-002").
				Build(),
		},
	}, {
		Name: "successful reconciliation with unsatisfied container reference",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.
				SpecBuildContainerRef(testContainer).
				Build(),
			imageNamesConfigMapGiven.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
			rtesting.NewTrackRequest(containerGiven.Get(), processorGiven.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.Unknown(),
					processorConditionStreamsReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "get container fails",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.
				SpecBuildContainerRef(testContainer).
				Build(),
			imageNamesConfigMapGiven.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
			rtesting.NewTrackRequest(containerGiven.Get(), processorGiven.Build(), scheme),
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Container"),
		},
		ShouldErr: true,
	}, {
		Name: "successful reconciliation with satisfied container reference",
		Key:  types.NamespacedName{Namespace: testNamespace, Name: testName},
		GivenObjects: []runtime.Object{
			processorGiven.
				SpecBuildContainerRef(testContainer).
				Build(),
			imageNamesConfigMapGiven.Build(),
			containerGiven.Get(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(imageNamesConfigMapGiven.Build(), processorGiven.Build(), scheme),
			rtesting.NewTrackRequest(containerGiven.Get(), processorGiven.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.ContainerNamed(testContainer, testCoreContainer(testContainerImage))
					pts.ContainerNamed("processor", processorCoreContainer)
				}).
				Build(),
			scaledObjectCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			processorGiven.
				StatusConditions(
					processorConditionDeploymentReady.Unknown(),
					processorConditionReady.Unknown(),
					processorConditionScaledObjectReady.True(),
					processorConditionStreamsReady.True(),
				).
				StatusLatestImage(testContainerImage).
				StatusDeploymentRef(testName + "-processor-001").
				StatusScaledObjectRef(testName + "-processor-002").
				Build(),
		},
	}}

	table.Test(t, scheme, func(t *testing.T, row *rtesting.Testcase, client client.Client, tracker tracker.Tracker, log logr.Logger) reconcile.Reconciler {
		return &ProcessorReconciler{
			Client:    client,
			Log:       log,
			Scheme:    scheme,
			Tracker:   tracker,
			Namespace: testNamespace,
		}
	})
}
