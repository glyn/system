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

package knative_test

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	buildv1alpha1 "github.com/projectriff/system/pkg/apis/build/v1alpha1"
	knativev1alpha1 "github.com/projectriff/system/pkg/apis/knative/v1alpha1"
	knativeservingv1 "github.com/projectriff/system/pkg/apis/thirdparty/knative/serving/v1"
	"github.com/projectriff/system/pkg/controllers/knative"
	rtesting "github.com/projectriff/system/pkg/controllers/testing"
	"github.com/projectriff/system/pkg/controllers/testing/builders"
	"github.com/projectriff/system/pkg/tracker"
)

func TestDeployerReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-deployer"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}
	testImagePrefix := "example.com/repo"
	testSha256 := "cf8b4c69d5460f88530e1c80b8856a70801f31c50b191c8413043ba9b160a43e"
	testImage := fmt.Sprintf("%s/%s@sha256:%s", testImagePrefix, testName, testSha256)
	testAddressURL := "http://internal.local"
	testURL := "http://example.com"

	deployerConditionConfigurationReady := builders.Condition().Type(knativev1alpha1.DeployerConditionConfigurationReady)
	deployerConditionReady := builders.Condition().Type(knativev1alpha1.DeployerConditionReady)
	deployerConditionRouteReady := builders.Condition().Type(knativev1alpha1.DeployerConditionRouteReady)

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = buildv1alpha1.AddToScheme(scheme)
	_ = knativev1alpha1.AddToScheme(scheme)
	_ = knativeservingv1.AddToScheme(scheme)

	testDeployer := builders.DeployerKnative().
		NamespaceName(testNamespace, testName)

	testApplication := builders.Application().
		NamespaceName(testNamespace, "my-application").
		StatusLatestImage(testImage)
	testFunction := builders.Function().
		NamespaceName(testNamespace, "my-function").
		StatusLatestImage(testImage)
	testContainer := builders.Container().
		NamespaceName(testNamespace, "my-container").
		StatusLatestImage(testImage)

	testConfigurationCreate := builders.KnativeConfiguration().
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Namespace(testNamespace)
			om.GenerateName("%s-deployer-", testName)
			om.ControlledBy(testDeployer.Build(), scheme)
			om.AddLabel(knativev1alpha1.DeployerLabelKey, testName)
			om.AddLabel("serving.knative.dev/visibility", "cluster-local")
		}).
		PodTemplateSpec(func(pts builders.PodTemplateSpec) {
			pts.AddLabel(knativev1alpha1.DeployerLabelKey, testName)
			pts.AddLabel("serving.knative.dev/visibility", "cluster-local")
		}).
		UserContainer(func(container *corev1.Container) {
			container.Image = testImage
		})
	testConfigurationGiven := testConfigurationCreate.
		ObjectMeta(func(om builders.ObjectMeta) {
			om.
				Name("%s001", om.Build().GenerateName).
				Generation(1)
		}).
		StatusObservedGeneration(1)

	testRouteCreate := builders.KnativeRoute().
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Namespace(testNamespace)
			om.Name(testName)
			om.ControlledBy(testDeployer.Build(), scheme)
			om.AddLabel(knativev1alpha1.DeployerLabelKey, testName)
			om.AddLabel("serving.knative.dev/visibility", "cluster-local")
		}).
		Traffic(
			knativeservingv1.TrafficTarget{
				ConfigurationName: fmt.Sprintf("%s-deployer-%s", testName, "001"),
				Percent:           rtesting.Int64Ptr(100),
			},
		)
	testRouteGiven := testRouteCreate.
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Generation(1)
		}).
		StatusObservedGeneration(1)

	table := rtesting.Table{{
		Name: "deployer does not exist",
		Key:  testKey,
	}, {
		Name: "ignore deleted deployer",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.Deleted(1)
				}).
				Build(),
		},
	}, {
		Name: "get deployer failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Deployer"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.Deleted(1)
				}).
				Build(),
		},
		ShouldErr: true,
	}, {
		Name: "create knative resources, from application",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ApplicationRef(testApplication.Build().Name).
				Build(),
			testApplication.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testApplication.Build(), testDeployer.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, from application, application not found",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ApplicationRef(testApplication.Build().Name).
				Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testApplication.Build(), testDeployer.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "create knative resources, from application, get application failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Application"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				ApplicationRef(testApplication.Build().Name).
				Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testApplication.Build(), testDeployer.Build(), scheme),
		},
	}, {
		Name: "create knative resources, from application, no latest",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ApplicationRef(testApplication.Build().Name).
				Build(),
			testApplication.
				StatusLatestImage("").
				Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testApplication.Build(), testDeployer.Build(), scheme),
		},
	}, {
		Name: "create knative resources, from function",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				FunctionRef(testFunction.Build().Name).
				Build(),
			testFunction.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testFunction.Build(), testDeployer.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, from function, function not found",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				FunctionRef(testFunction.Build().Name).
				Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testFunction.Build(), testDeployer.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "create knative resources, from function, get function failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Function"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				FunctionRef(testFunction.Build().Name).
				Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testFunction.Build(), testDeployer.Build(), scheme),
		},
	}, {
		Name: "create knative resources, from function, no latest",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				FunctionRef(testFunction.Build().Name).
				Build(),
			testFunction.
				StatusLatestImage("").
				Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testFunction.Build(), testDeployer.Build(), scheme),
		},
	}, {
		Name: "create knative resources, from container",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ContainerRef(testContainer.Get().Name).
				Build(),
			testContainer.Get(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testContainer.Get(), testDeployer.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, from container, container not found",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ContainerRef(testContainer.Get().Name).
				Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testContainer.Get(), testDeployer.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "create knative resources, from container, get container failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Container"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				ContainerRef(testContainer.Get().Name).
				Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testContainer.Get(), testDeployer.Build(), scheme),
		},
	}, {
		Name: "create knative resources, from container, no latest",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ContainerRef(testContainer.Get().Name).
				Build(),
			testContainer.
				StatusLatestImage("").
				Get(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testContainer.Get(), testDeployer.Build(), scheme),
		},
	}, {
		Name: "create knative resources, from image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
		},
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, create configuration failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Configuration"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
		},
		ShouldErr: true,
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				Build(),
		},
	}, {
		Name: "create knative resources, create route failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Route"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
		},
		ShouldErr: true,
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, route exists",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Route", rtesting.InduceFailureOpts{
				Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
			}),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
		},
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.False().Reason("NotOwned", `There is an existing Route "test-deployer" that the Deployer does not own.`),
					deployerConditionRouteReady.False().Reason("NotOwned", `There is an existing Route "test-deployer" that the Deployer does not own.`),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, delete extra configurations",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				NamespaceName(testNamespace, "extra-configuration-1").
				Build(),
			testConfigurationGiven.
				NamespaceName(testNamespace, "extra-configuration-2").
				Build(),
		},
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "serving.knative.dev", Kind: "Configuration", Namespace: testNamespace, Name: "extra-configuration-1"},
			{Group: "serving.knative.dev", Kind: "Configuration", Namespace: testNamespace, Name: "extra-configuration-2"},
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, delete extra configurations, delete failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Configuration"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				NamespaceName(testNamespace, "extra-configuration-1").
				Build(),
			testConfigurationGiven.
				NamespaceName(testNamespace, "extra-configuration-2").
				Build(),
		},
		ShouldErr: true,
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "serving.knative.dev", Kind: "Configuration", Namespace: testNamespace, Name: "extra-configuration-1"},
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				Build(),
		},
	}, {
		Name: "create knative resources, delete extra routes",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testRouteGiven.
				NamespaceName(testNamespace, "extra-route-1").
				Build(),
			testRouteGiven.
				NamespaceName(testNamespace, "extra-route-2").
				Build(),
		},
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
			testRouteCreate.Build(),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "serving.knative.dev", Kind: "Route", Namespace: testNamespace, Name: "extra-route-1"},
			{Group: "serving.knative.dev", Kind: "Route", Namespace: testNamespace, Name: "extra-route-2"},
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "create knative resources, delete extra routes, delete failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Route"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testRouteGiven.
				NamespaceName(testNamespace, "extra-route-1").
				Build(),
			testRouteGiven.
				NamespaceName(testNamespace, "extra-route-2").
				Build(),
		},
		ShouldErr: true,
		ExpectCreates: []runtime.Object{
			testConfigurationCreate.Build(),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "serving.knative.dev", Kind: "Route", Namespace: testNamespace, Name: "extra-route-1"},
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "update configuration",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				UserContainer(func(container *corev1.Container) {
					container.Image = "bogus"
				}).
				Build(),
			testRouteGiven.Build(),
		},
		ExpectUpdates: []runtime.Object{
			testConfigurationGiven.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "update configuration, listing failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "ConfigurationList"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				UserContainer(func(container *corev1.Container) {
					container.Image = "bogus"
				}).
				Build(),
			testRouteGiven.Build(),
		},
		ShouldErr: true,
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				Build(),
		},
	}, {
		Name: "update configuration, update failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Configuration"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				UserContainer(func(container *corev1.Container) {
					container.Image = "bogus"
				}).
				Build(),
			testRouteGiven.Build(),
		},
		ShouldErr: true,
		ExpectUpdates: []runtime.Object{
			testConfigurationGiven.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				Build(),
		},
	}, {
		Name: "update route",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.Build(),
			testRouteGiven.
				Traffic().
				Build(),
		},
		ExpectUpdates: []runtime.Object{
			testRouteGiven.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "update route, listing failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "RouteList"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.Build(),
			testRouteGiven.
				Traffic().
				Build(),
		},
		ShouldErr: true,
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "update route, update failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Route"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.Build(),
			testRouteGiven.
				Traffic().
				Build(),
		},
		ShouldErr: true,
		ExpectUpdates: []runtime.Object{
			testRouteGiven.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "update status failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Deployer"),
		},
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.Build(),
			testRouteGiven.Build(),
		},
		ShouldErr: true,
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "update knative resources, copy annotations and labels",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddAnnotation("test-annotation", "test-annotation-value")
					om.AddLabel("test-label", "test-label-value")
				}).
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.AddAnnotation("test-annotation-pts", "test-annotation-value")
					pts.AddLabel("test-label-pts", "test-label-value")
				}).
				Image(testImage).
				Build(),
			testConfigurationGiven.Build(),
			testRouteGiven.Build(),
		},
		ExpectUpdates: []runtime.Object{
			testConfigurationGiven.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddAnnotation("test-annotation", "test-annotation-value")
					om.AddLabel("test-label", "test-label-value")
				}).
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.AddAnnotation("test-annotation", "test-annotation-value")
					pts.AddLabel("test-label", "test-label-value")
				}).
				Build(),
			testRouteGiven.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel("test-label", "test-label-value")
				}).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "update knative resources, with scale",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				MinScale(1).
				MaxScale(2).
				Build(),
			testConfigurationGiven.Build(),
			testRouteGiven.Build(),
		},
		ExpectUpdates: []runtime.Object{
			testConfigurationGiven.
				// TODO figure out which annotation is actually impactful
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddAnnotation("autoscaling.knative.dev/minScale", "1")
					om.AddAnnotation("autoscaling.knative.dev/maxScale", "2")
				}).
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.AddAnnotation("autoscaling.knative.dev/minScale", "1")
					pts.AddAnnotation("autoscaling.knative.dev/maxScale", "2")
				}).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionRouteReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				Build(),
		},
	}, {
		Name: "ready",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				StatusConditions(
					builders.Condition().Type(knativeservingv1.ConfigurationConditionReady).True(),
				).
				Build(),
			testRouteGiven.
				StatusConditions(
					builders.Condition().Type(knativeservingv1.RouteConditionReady).True(),
				).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.True(),
					deployerConditionReady.True(),
					deployerConditionRouteReady.True(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
	}, {
		Name: "not ready, configuration",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				StatusConditions(
					builders.Condition().Type(knativeservingv1.ConfigurationConditionReady).False().Reason("TestReason", "a human readable message"),
				).
				Build(),
			testRouteGiven.
				StatusReady().
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.False().Reason("TestReason", "a human readable message"),
					deployerConditionReady.False().Reason("TestReason", "a human readable message"),
					deployerConditionRouteReady.True(),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
	}, {
		Name: "not ready, route",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			testDeployer.
				Image(testImage).
				Build(),
			testConfigurationGiven.
				StatusReady().
				Build(),
			testRouteGiven.
				StatusConditions(
					builders.Condition().Type(knativeservingv1.RouteConditionReady).False().Reason("TestReason", "a human readable message"),
				).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			testDeployer.
				StatusConditions(
					deployerConditionConfigurationReady.True(),
					deployerConditionReady.False().Reason("TestReason", "a human readable message"),
					deployerConditionRouteReady.False().Reason("TestReason", "a human readable message"),
				).
				StatusLatestImage(testImage).
				StatusConfigurationRef(testConfigurationGiven.Build().Name).
				StatusRouteRef(testRouteGiven.Build().Name).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
	}}

	table.Test(t, scheme, func(t *testing.T, row *rtesting.Testcase, client client.Client, tracker tracker.Tracker, log logr.Logger) reconcile.Reconciler {
		return &knative.DeployerReconciler{
			Client:  client,
			Log:     log,
			Scheme:  scheme,
			Tracker: tracker,
		}
	})
}
