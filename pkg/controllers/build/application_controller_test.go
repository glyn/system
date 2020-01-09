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

package build_test

import (
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/projectriff/system/pkg/apis"
	buildv1alpha1 "github.com/projectriff/system/pkg/apis/build/v1alpha1"
	kpackbuildv1alpha1 "github.com/projectriff/system/pkg/apis/thirdparty/kpack/build/v1alpha1"
	"github.com/projectriff/system/pkg/controllers/build"
	rtesting "github.com/projectriff/system/pkg/controllers/testing"
	"github.com/projectriff/system/pkg/controllers/testing/builders"
	"github.com/projectriff/system/pkg/tracker"
)

func TestApplicationReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-application"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}
	testImagePrefix := "example.com/repo"
	testGitUrl := "git@example.com:repo.git"
	testGitRevision := "master"
	testSha256 := "cf8b4c69d5460f88530e1c80b8856a70801f31c50b191c8413043ba9b160a43e"
	testConditionReason := "TestReason"
	testConditionMessage := "meaningful, yet concise"
	testLabelKey := "test-label-key"
	testLabelValue := "test-label-value"
	testBuildCacheName := "test-build-cache-000"

	applicationConditionImageResolved := builders.Condition().Type(buildv1alpha1.ApplicationConditionImageResolved)
	applicationConditionKpackImageReady := builders.Condition().Type(buildv1alpha1.ApplicationConditionKpackImageReady)
	applicationConditionReady := builders.Condition().Type(buildv1alpha1.ApplicationConditionReady)

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kpackbuildv1alpha1.AddToScheme(scheme)
	_ = buildv1alpha1.AddToScheme(scheme)

	appMinimal := builders.Application().
		NamespaceName(testNamespace, testName)
	appValid := appMinimal.
		Image("%s/%s", testImagePrefix, testName).
		SourceGit(testGitUrl, testGitRevision)

	kpackImageCreate := builders.KpackImage().
		ObjectMeta(func(om builders.ObjectMeta) {
			om.Namespace(testNamespace).
				GenerateName("%s-application-", testName).
				AddLabel(buildv1alpha1.ApplicationLabelKey, testName).
				ControlledBy(appMinimal.Build(), scheme)
		}).
		Tag("%s/%s", testImagePrefix, testName).
		ApplicationBuilder().
		SourceGit(testGitUrl, testGitRevision)
	kpackImageGiven := kpackImageCreate.
		ObjectMeta(func(om builders.ObjectMeta) {
			om.
				Name("%s-application-001", testName).
				Generation(1)
		}).
		StatusObservedGeneration(1)

	cmImagePrefix := builders.ConfigMap().
		NamespaceName(testNamespace, "riff-build").
		AddData("default-image-prefix", "")

	table := rtesting.Table{{
		Name: "application does not exist",
		Key:  testKey,
	}, {
		Name: "ignore deleted application",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.Deleted(1)
				}).
				Build(),
		},
	}, {
		Name: "application get error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Application"),
		},
		ShouldErr: true,
	}, {
		Name: "create kpack image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.Build(),
		},
		ExpectCreates: []runtime.Object{
			kpackImageCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef("%s-application-001", testName).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "create kpack image, build cache",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.
				BuildCache("1Gi").
				Build(),
		},
		ExpectCreates: []runtime.Object{
			kpackImageCreate.
				BuildCache("1Gi").
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef("%s-application-001", testName).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "create kpack image, propagating labels",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				Build(),
		},
		ExpectCreates: []runtime.Object{
			kpackImageCreate.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef("%s-application-001", testName).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "default image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			cmImagePrefix.
				AddData("default-image-prefix", testImagePrefix).
				Build(),
			appMinimal.
				SourceGit(testGitUrl, testGitRevision).
				Build(),
		},
		ExpectCreates: []runtime.Object{
			kpackImageCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef("%s-application-001", testName).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "default image, missing",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appMinimal.
				SourceGit(testGitUrl, testGitRevision).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.False().Reason("DefaultImagePrefixMissing", "missing default image prefix"),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.False().Reason("DefaultImagePrefixMissing", "missing default image prefix"),
				).
				Build(),
		},
		ShouldErr: true,
	}, {
		Name: "default image, undefined",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			cmImagePrefix.Build(),
			appMinimal.
				SourceGit(testGitUrl, testGitRevision).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.False().Reason("DefaultImagePrefixMissing", "missing default image prefix"),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.False().Reason("DefaultImagePrefixMissing", "missing default image prefix"),
				).
				Build(),
		},
		ShouldErr: true,
	}, {
		Name: "default image, error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "ConfigMap"),
		},
		GivenObjects: []runtime.Object{
			cmImagePrefix.Build(),
			appMinimal.
				SourceGit(testGitUrl, testGitRevision).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.False().Reason("ImageInvalid", "inducing failure for get ConfigMap"),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.False().Reason("ImageInvalid", "inducing failure for get ConfigMap"),
				).
				Build(),
		},
		ShouldErr: true,
	}, {
		Name: "kpack image ready",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.Build(),
			kpackImageGiven.
				StatusReady().
				StatusLatestImage("%s/%s@sha256:%s", testImagePrefix, testName, testSha256).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.True(),
					applicationConditionReady.True(),
				).
				StatusKpackImageRef(kpackImageGiven.Build().Name).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				StatusLatestImage("%s/%s@sha256:%s", testImagePrefix, testName, testSha256).
				Build(),
		},
	}, {
		Name: "kpack image ready, build cache",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.Build(),
			kpackImageGiven.
				StatusReady().
				StatusBuildCacheName(testBuildCacheName).
				StatusLatestImage("%s/%s@sha256:%s", testImagePrefix, testName, testSha256).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.True(),
					applicationConditionReady.True(),
				).
				StatusKpackImageRef(kpackImageGiven.Build().Name).
				StatusBuildCacheRef(testBuildCacheName).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				StatusLatestImage("%s/%s@sha256:%s", testImagePrefix, testName, testSha256).
				Build(),
		},
	}, {
		Name: "kpack image not-ready",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.Build(),
			kpackImageGiven.
				StatusConditions(
					builders.Condition().Type(apis.ConditionReady).False().Reason(testConditionReason, testConditionMessage),
				).
				StatusLatestImage("%s/%s@sha256:%s", testImagePrefix, testName, testSha256).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.False().Reason(testConditionReason, testConditionMessage),
					applicationConditionReady.False().Reason(testConditionReason, testConditionMessage),
				).
				StatusKpackImageRef(kpackImageGiven.Build().Name).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				StatusLatestImage("%s/%s@sha256:%s", testImagePrefix, testName, testSha256).
				Build(),
		},
	}, {
		Name: "kpack image create error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("create", "Image"),
		},
		GivenObjects: []runtime.Object{
			appValid.Build(),
		},
		ShouldErr: true,
		ExpectCreates: []runtime.Object{
			kpackImageCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appValid.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "kpack image update, spec",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.Build(),
			kpackImageGiven.
				SourceGit(testGitUrl, "bogus").
				Build(),
		},
		ExpectUpdates: []runtime.Object{
			kpackImageGiven.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appValid.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef(kpackImageGiven.Build().Name).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "kpack image update, labels",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				Build(),
			kpackImageGiven.Build(),
		},
		ExpectUpdates: []runtime.Object{
			kpackImageGiven.
				ObjectMeta(func(om builders.ObjectMeta) {
					om.AddLabel(testLabelKey, testLabelValue)
				}).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appValid.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef(kpackImageGiven.Build().Name).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "kpack image update, fails",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Image"),
		},
		GivenObjects: []runtime.Object{
			appValid.Build(),
			kpackImageGiven.
				SourceGit(testGitUrl, "bogus").
				Build(),
		},
		ShouldErr: true,
		ExpectUpdates: []runtime.Object{
			kpackImageGiven.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appValid.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "kpack image list error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "ImageList"),
		},
		GivenObjects: []runtime.Object{
			appValid.Build(),
		},
		ShouldErr: true,
		ExpectStatusUpdates: []runtime.Object{
			appValid.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "application status update error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("update", "Application"),
		},
		GivenObjects: []runtime.Object{
			appValid.Build(),
		},
		ExpectCreates: []runtime.Object{
			kpackImageCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef("%s-application-001", testName).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
		ShouldErr: true,
	}, {
		Name: "delete extra kpack image",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appValid.Build(),
			kpackImageGiven.
				NamespaceName(testNamespace, "extra1").
				Build(),
			kpackImageGiven.
				NamespaceName(testNamespace, "extra2").
				Build(),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "build.pivotal.io", Kind: "Image", Namespace: testNamespace, Name: "extra1"},
			{Group: "build.pivotal.io", Kind: "Image", Namespace: testNamespace, Name: "extra2"},
		},
		ExpectCreates: []runtime.Object{
			kpackImageCreate.Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusKpackImageRef("%s-application-001", testName).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "delete extra kpack image, fails",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Image"),
		},
		GivenObjects: []runtime.Object{
			appValid.Build(),
			kpackImageGiven.
				NamespaceName(testNamespace, "extra1").
				Build(),
			kpackImageGiven.
				NamespaceName(testNamespace, "extra2").
				Build(),
		},
		ShouldErr: true,
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "build.pivotal.io", Kind: "Image", Namespace: testNamespace, Name: "extra1"},
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "local build",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appMinimal.
				Image("%s/%s", testImagePrefix, testName).
				Build(),
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.True(),
					applicationConditionReady.True(),
				).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				// TODO resolve to a digest
				StatusLatestImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "local build, removes existing build",
		Key:  testKey,
		GivenObjects: []runtime.Object{
			appMinimal.
				Image("%s/%s", testImagePrefix, testName).
				Build(),
			kpackImageGiven.Build(),
		},
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "build.pivotal.io", Kind: "Image", Namespace: kpackImageGiven.Build().Namespace, Name: kpackImageGiven.Build().Name},
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.True(),
					applicationConditionReady.True(),
				).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				// TODO resolve to a digest
				StatusLatestImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}, {
		Name: "local build, removes existing build, error",
		Key:  testKey,
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("delete", "Image"),
		},
		GivenObjects: []runtime.Object{
			appMinimal.
				Image("%s/%s", testImagePrefix, testName).
				Build(),
			kpackImageGiven.Build(),
		},
		ShouldErr: true,
		ExpectDeletes: []rtesting.DeleteRef{
			{Group: "build.pivotal.io", Kind: "Image", Namespace: kpackImageGiven.Build().Namespace, Name: kpackImageGiven.Build().Name},
		},
		ExpectStatusUpdates: []runtime.Object{
			appMinimal.
				StatusConditions(
					applicationConditionImageResolved.True(),
					applicationConditionKpackImageReady.Unknown(),
					applicationConditionReady.Unknown(),
				).
				StatusTargetImage("%s/%s", testImagePrefix, testName).
				Build(),
		},
	}}

	table.Test(t, scheme, func(t *testing.T, row *rtesting.Testcase, client client.Client, tracker tracker.Tracker, log logr.Logger) reconcile.Reconciler {
		return &build.ApplicationReconciler{
			Client: client,
			Scheme: scheme,
			Log:    log,
		}
	})
}
