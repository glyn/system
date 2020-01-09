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

package core_test

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	buildv1alpha1 "github.com/projectriff/system/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/projectriff/system/pkg/apis/core/v1alpha1"
	"github.com/projectriff/system/pkg/controllers/core"
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
	testImage := fmt.Sprintf("%s@sha256:%s", testImagePrefix, testSha256)
	testConditionReason := "TestReason"
	testConditionMessage := "meaningful, yet concise"
	testDomain := "example.com"
	testHost := fmt.Sprintf("%s.%s.%s", testName, testNamespace, testDomain)
	testURL := fmt.Sprintf("http://%s", testHost)
	testAddressURL := fmt.Sprintf("http://%s.%s.svc.cluster.local", testName, testNamespace)
	testLabelKey := "test-label-key"
	testLabelValue := "test-label-value"

	deployerConditionDeploymentReady := builders.Condition().Type(corev1alpha1.DeployerConditionDeploymentReady)
	deployerConditionIngressReady := builders.Condition().Type(corev1alpha1.DeployerConditionIngressReady).Info()
	deployerConditionReady := builders.Condition().Type(corev1alpha1.DeployerConditionReady)
	deployerConditionServiceReady := builders.Condition().Type(corev1alpha1.DeployerConditionServiceReady)

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = buildv1alpha1.AddToScheme(scheme)
	_ = corev1alpha1.AddToScheme(scheme)

	deployerMinimal := builders.DeployerCore().
		NamespaceName(testNamespace, testName)
	deployerValid := deployerMinimal.
		Image(testImage).
		IngressPolicy(corev1alpha1.IngressPolicyClusterLocal)

	deploymentCreate := builders.Deployment().
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Namespace(testNamespace)
			om.GenerateName("%s-deployer-", deployerMinimal.Build().Name)
			om.AddLabel(corev1alpha1.DeployerLabelKey, deployerMinimal.Build().Name)
			om.ControlledBy(deployerMinimal.Build(), scheme)
		}).
		AddSelectorLabel(corev1alpha1.DeployerLabelKey, deployerMinimal.Build().Name).
		HandlerContainer(func(container *corev1.Container) {
			container.Image = testImage
			container.Ports = []corev1.ContainerPort{
				{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			}
			container.Env = []corev1.EnvVar{
				{Name: "PORT", Value: "8080"},
			}
			container.ReadinessProbe = &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8080),
					},
				},
			}
		})
	deploymentGiven := deploymentCreate.
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Name("%s%s", om.Build().GenerateName, "000")
			om.Created(1)
		})

	serviceCreate := builders.Service().
		NamespaceName(testNamespace, testName).
		ObjectMeta(func(om builders.ObjectMeta) {
			om.AddLabel(corev1alpha1.DeployerLabelKey, deployerMinimal.Build().Name)
			om.ControlledBy(deployerMinimal.Build(), scheme)
		}).
		AddSelectorLabel(corev1alpha1.DeployerLabelKey, deployerMinimal.Build().Name).
		Ports(
			corev1.ServicePort{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
		)
	serviceGiven := serviceCreate.
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Created(1)
			om.ControlledBy(deployerMinimal.Build(), scheme)
		})

	ingressCreate := builders.Ingress().
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Namespace(testNamespace)
			om.GenerateName("%s-deployer-", deployerMinimal.Build().Name)
			om.AddLabel(corev1alpha1.DeployerLabelKey, deployerMinimal.Build().Name)
			om.ControlledBy(deployerMinimal.Build(), scheme)
		}).
		HostToService(testHost, serviceGiven.Build().Name)
	ingressGiven := ingressCreate.
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Name("%s%s", om.Build().GenerateName, "000")
			om.Created(1)
		})

	testApplication := builders.Application().
		NamespaceName(testNamespace, "my-application").
		StatusLatestImage(testImage)
	testFunction := builders.Function().
		NamespaceName(testNamespace, "my-function").
		StatusLatestImage(testImage)
	testContainer := builders.Container().
		NamespaceName(testNamespace, "my-container").
		StatusLatestImage(testImage)

	testSettings := builders.ConfigMap().
		NamespaceName("riff-system", "riff-core-settings").
		AddData("defaultDomain", "example.com")

	table := rtesting.Table{{
		Name: "deployer does not exist",
		Key:  testKey,
	}, {
		Name: "ignore deleted deployer",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerValid.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.Deleted(1)
				}).
				Build(),
		},
	}, {
		Name: "deployer get error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Deployer"),
		},
		ShouldErr: true,
	}, {
		Name: "create resources, from application",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				ApplicationRef(testApplication.Build().Name).
				Build(),
			testApplication.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testApplication.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
			serviceCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "create resources, from application, application not found",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				ApplicationRef(testApplication.Build().Name).
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testApplication.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "create resources, from application, no latest image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				ApplicationRef(testApplication.Build().Name).
				Build(),
			testApplication.
				StatusLatestImage("").
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testApplication.Build(), deployerMinimal.Build(), scheme),
		},
	}, {
		Name: "create resources, from function",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				FunctionRef(testFunction.Build().Name).
				Build(),
			testFunction.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testFunction.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
			serviceCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "create resources, from function, function not found",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				FunctionRef(testFunction.Build().Name).
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testFunction.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "create resources, from function, no latest image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				FunctionRef(testFunction.Build().Name).
				Build(),
			testFunction.
				StatusLatestImage("").
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testFunction.Build(), deployerMinimal.Build(), scheme),
		},
	}, {
		Name: "create resources, from container",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				ContainerRef(testContainer.Get().Name).
				Build(),
			testContainer.Get(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testContainer.Get(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
			serviceCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "create resources, from container, container not found",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				ContainerRef(testContainer.Get().Name).
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testContainer.Get(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.Unknown(),
				).
				Build(),
		},
	}, {
		Name: "create resources, from container, no latest image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				ContainerRef(testContainer.Get().Name).
				Build(),
			testContainer.
				StatusLatestImage("").
				Get(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
			rtesting.NewTrackRequest(testContainer.Get(), deployerMinimal.Build(), scheme),
		},
	}, {
		Name: "create resources, from image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
			serviceCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "create deployment, error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Deployment"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.Unknown(),
				).
				StatusLatestImage(testImage).
				Build(),
		},
	}, {
		Name: "create service, error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Service"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
			serviceCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.Unknown(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				Build(),
		},
	}, {
		Name: "create service, conflicted",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Service", rtesting.InduceFailureOpts{
				Error: apierrs.NewAlreadyExists(schema.GroupResource{}, testName),
			}),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
			serviceCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.False().Reason("NotOwned", `There is an existing Service "test-deployer" that the Deployer does not own.`),
					deployerConditionServiceReady.False().Reason("NotOwned", `There is an existing Service "test-deployer" that the Deployer does not own.`),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				Build(),
		},
	}, {
		Name: "update deployment",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.
				HandlerContainer(func(container *corev1.Container) {
					// change to reverse
					container.Env = nil
				}).
				Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectUpdates: []runtime.Object{
			deploymentGiven.Build(),
		},
	}, {
		Name: "update deployment, update error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Deployment"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.
				HandlerContainer(func(container *corev1.Container) {
					// change to reverse
					container.Env = nil
				}).
				Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectUpdates: []runtime.Object{
			deploymentGiven.Build(),
		},
	}, {
		Name: "update deployment, list deployments failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "DeploymentList"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.
				HandlerContainer(func(container *corev1.Container) {
					// change to reverse
					container.Env = nil
				}).
				Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
	}, {
		Name: "update service",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.
				// change to reverse
				Ports().
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectUpdates: []runtime.Object{
			serviceGiven.Build(),
		},
	}, {
		Name: "update service, update error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Service"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.
				// change to reverse
				Ports().
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectUpdates: []runtime.Object{
			serviceGiven.Build(),
		},
	}, {
		Name: "update service, list services failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "ServiceList"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.
				// change to reverse
				Ports().
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
	}, {
		Name: "cleanup extra deployments",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.
				NamespaceName(testNamespace, "extra-deployment-1").
				Build(),
			deploymentGiven.
				NamespaceName(testNamespace, "extra-deployment-2").
				Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "apps", Kind: "Deployment", Namespace: testNamespace, Name: "extra-deployment-1"},
			{Group: "apps", Kind: "Deployment", Namespace: testNamespace, Name: "extra-deployment-2"},
		},
		ExpectCreates: []runtime.Object{
			deploymentCreate.Build(),
		},
	}, {
		Name: "cleanup extra deployments, delete deployment failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Deployment"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-001", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.
				NamespaceName(testNamespace, "extra-deployment-1").
				Build(),
			deploymentGiven.
				NamespaceName(testNamespace, "extra-deployment-2").
				Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "apps", Kind: "Deployment", Namespace: testNamespace, Name: "extra-deployment-1"},
		},
	}, {
		Name: "cleanup extra services",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.
				NamespaceName(testNamespace, "extra-service-1").
				Build(),
			serviceGiven.
				NamespaceName(testNamespace, "extra-service-2").
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Kind: "Service", Namespace: testNamespace, Name: "extra-service-1"},
			{Kind: "Service", Namespace: testNamespace, Name: "extra-service-2"},
		},
		ExpectCreates: []runtime.Object{
			serviceCreate.Build(),
		},
	}, {
		Name: "cleanup extra services, delete service failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Service"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.
				NamespaceName(testNamespace, "extra-service-1").
				Build(),
			serviceGiven.
				NamespaceName(testNamespace, "extra-service-2").
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Kind: "Service", Namespace: testNamespace, Name: "extra-service-1"},
		},
	}, {
		Name: "create ingress",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			ingressCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.Unknown().Reason("IngressNotConfigured", "Ingress has not yet been reconciled."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusIngressRef("%s-deployer-001", testName).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
	}, {
		Name: "create ingress, create failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Ingress"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectCreates: []runtime.Object{
			ingressCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL(testAddressURL).
				Build(),
		},
	}, {
		Name: "delete ingress",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyClusterLocal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.Build(),
			testSettings.
				AddData("defaultDomain", "not.example.com").
				Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "networking.k8s.io", Kind: "Ingress", Namespace: testNamespace, Name: ingressGiven.Build().Name},
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL(testAddressURL).
				Build(),
		},
	}, {
		Name: "delete ingress, delete failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Ingress"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyClusterLocal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.Build(),
			testSettings.
				AddData("defaultDomain", "not.example.com").
				Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "networking.k8s.io", Kind: "Ingress", Namespace: testNamespace, Name: ingressGiven.Build().Name},
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL(testAddressURL).
				Build(),
		},
	}, {
		Name: "update ingress",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.Build(),
			testSettings.
				AddData("defaultDomain", "not.example.com").
				Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectUpdates: []runtime.Object{
			ingressGiven.
				HostToService(fmt.Sprintf("%s.%s.%s", testName, testNamespace, "not.example.com"), serviceGiven.Build().Name).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.Unknown().Reason("IngressNotConfigured", "Ingress has not yet been reconciled."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusIngressRef("%s-deployer-000", testName).
				StatusAddressURL(testAddressURL).
				StatusURL("http://%s.%s.%s", testName, testNamespace, "not.example.com").
				Build(),
		},
	}, {
		Name: "update ingress, update failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Ingress"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.Build(),
			testSettings.
				AddData("defaultDomain", "not.example.com").
				Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectUpdates: []runtime.Object{
			ingressGiven.
				HostToService(fmt.Sprintf("%s.%s.%s", testName, testNamespace, "not.example.com"), serviceGiven.Build().Name).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL(testAddressURL).
				Build(),
		},
	}, {
		Name: "remove extra ingress",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.
				NamespaceName(testNamespace, "extra-ingress-1").
				Build(),
			ingressGiven.
				NamespaceName(testNamespace, "extra-ingress-2").
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "networking.k8s.io", Kind: "Ingress", Namespace: testNamespace, Name: "extra-ingress-1"},
			{Group: "networking.k8s.io", Kind: "Ingress", Namespace: testNamespace, Name: "extra-ingress-2"},
		},
		ExpectCreates: []runtime.Object{
			ingressCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.Unknown().Reason("IngressNotConfigured", "Ingress has not yet been reconciled."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusIngressRef("%s-deployer-001", testName).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
	}, {
		Name: "remove extra ingress, listing failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "IngressList"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.
				NamespaceName(testNamespace, "extra-ingress-1").
				Build(),
			ingressGiven.
				NamespaceName(testNamespace, "extra-ingress-2").
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "remove extra ingress, delete failed",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Ingress"),
		},
		GivenObjects: []runtime.Object{
			deployerMinimal.
				Image(testImage).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.
				NamespaceName(testNamespace, "extra-ingress-1").
				Build(),
			ingressGiven.
				NamespaceName(testNamespace, "extra-ingress-2").
				Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "networking.k8s.io", Kind: "Ingress", Namespace: testNamespace, Name: "extra-ingress-1"},
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "propagate labels",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Image(testImage).
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.Unknown().Reason("IngressNotConfigured", "Ingress has not yet been reconciled."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusIngressRef(ingressGiven.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				StatusURL(testURL).
				Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			ingressGiven.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectUpdates: []runtime.Object{
			deploymentGiven.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				PodTemplateSpec(func(pts builders.PodTemplateSpec) {
					pts.AddLabel(testLabelKey, testLabelValue)
				}).
				Build(),
			serviceGiven.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				Build(),
			ingressGiven.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				Build(),
		},
	}, {
		Name: "ready",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerValid.Build(),
			deploymentGiven.
				StatusConditions(
					builders.Condition().Type("Available").True(),
					builders.Condition().Type("Progressing").Unknown(),
				).
				Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.True(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.True(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "ready, with ingress",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerValid.
				IngressPolicy(corev1alpha1.IngressPolicyExternal).
				Build(),
			deploymentGiven.
				StatusConditions(
					builders.Condition().Type("Available").True(),
					builders.Condition().Type("Progressing").Unknown(),
				).
				Build(),
			serviceGiven.Build(),
			ingressGiven.
				StatusLoadBalancer(
					corev1.LoadBalancerIngress{
						Hostname: testHost,
					},
				).
				Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.True(),
					deployerConditionIngressReady.True(),
					deployerConditionReady.True(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusIngressRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusAddressURL(testAddressURL).
				StatusURL(testURL).
				Build(),
		},
	}, {
		Name: "not ready",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerValid.Build(),
			deploymentGiven.
				StatusConditions(
					builders.Condition().Type("Available").False().Reason(testConditionReason, testConditionMessage),
					builders.Condition().Type("Progressing").Unknown(),
				).
				Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.False().Reason(testConditionReason, testConditionMessage),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.False().Reason(testConditionReason, testConditionMessage),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "update status error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Deployer"),
		},
		GivenObjects: []runtime.Object{
			deployerValid.Build(),
			deploymentGiven.Build(),
			serviceGiven.Build(),
			testSettings.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionIngressReady.False().Reason("IngressNotRequired", "Ingress resource is not required."),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.True(),
				).
				StatusLatestImage(testImage).
				StatusDeploymentRef("%s-deployer-000", deployerMinimal.Build().Name).
				StatusServiceRef(deployerMinimal.Build().Name).
				StatusAddressURL("http://%s.%s.svc.cluster.local", serviceCreate.Build().Name, serviceCreate.Build().Namespace).
				Build(),
		},
	}, {
		Name: "settings not found",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			deployerMinimal.Build(),
		},
		ShouldErr: true,
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(testSettings.Build(), deployerMinimal.Build(), scheme),
		},
		ExpectStatusUpdates: []runtime.Object{
			deployerMinimal.
				StatusConditions(
					deployerConditionDeploymentReady.Unknown(),
					deployerConditionReady.Unknown(),
					deployerConditionServiceReady.Unknown(),
				).
				Build(),
		},
	}}

	table.Test(t, scheme, func(t *testing.T, row *rtesting.Testcase, client client.Client, tracker tracker.Tracker, log logr.Logger) reconcile.Reconciler {
		return &core.DeployerReconciler{
			Client:  client,
			Scheme:  scheme,
			Log:     log,
			Tracker: tracker,
		}
	})
}
