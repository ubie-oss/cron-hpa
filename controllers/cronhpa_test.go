/*
Copyright 2021 Daisuke Taniwaki.

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

package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	cronhpav1alpha1 "github.com/ubie-oss/cron-hpa/api/v1alpha1"
	"github.com/ubie-oss/cron-hpa/test"
)

func TestNewHPA(t *testing.T) {
	cronHPAManifest := `
apiVersion: cron-hpa.ubie-oss.github.com/v1alpha1
kind: CronHorizontalPodAutoscaler
metadata:
  name: cron-hpa-sample
  namespace: default
spec:
  template:
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: cron-hpa-nginx
      minReplicas: 1
      maxReplicas: 10
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 50
  scheduledPatches:
  - name: one
    schedule: "0,10,20,30,40,50 * * * *"
    timezone: "Asia/Tokyo"
    patch:
      minReplicas: 3
      maxReplicas: 15
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 30
`

	cronhpa := &CronHorizontalPodAutoscaler{}
	err := yaml.Unmarshal([]byte(cronHPAManifest), cronhpa.ToCompatible())
	require.NoError(t, err)

	defaultHPAManifest := `
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: cron-hpa-sample
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: cron-hpa-nginx
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
`

	defaultHPA := &autoscalingv2.HorizontalPodAutoscaler{}
	err = yaml.Unmarshal([]byte(defaultHPAManifest), defaultHPA)
	require.NoError(t, err)
	hpa, err := cronhpa.NewHPA("")
	require.NoError(t, err)
	fmt.Printf("kind %s %s\n", defaultHPA.Kind, hpa.Kind)
	fmt.Printf("kind %s %s\n", defaultHPA.Kind, hpa.Kind)
	if !assert.Equal(t, defaultHPA, hpa) {
		t.FailNow()
	}

	withPatchHPAManifest := `
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: cron-hpa-sample
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: cron-hpa-nginx
  minReplicas: 3
  maxReplicas: 15
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 30
`

	withPatchHPA := &autoscalingv2.HorizontalPodAutoscaler{}
	err = yaml.Unmarshal([]byte(withPatchHPAManifest), withPatchHPA)
	require.NoError(t, err)

	hpa, err = cronhpa.NewHPA("one")
	require.NoError(t, err)
	if !assert.Equal(t, withPatchHPA, hpa) {
		t.FailNow()
	}
}

func TestGetCurrentPatchName(t *testing.T) {
	ctx := context.TODO()

	cronHPAManifest := `
apiVersion: cron-hpa.ubie-oss.github.com/v1alpha1
kind: CronHorizontalPodAutoscaler
metadata:
  name: cron-hpa-sample
  namespace: default
spec:
  template:
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: cron-hpa-nginx
      minReplicas: 1
      maxReplicas: 10
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 50
  scheduledPatches:
  - name: weekday
    schedule: "0 0 * 10 mon-fri"
    timezone: "Asia/Tokyo"
  - name: weekend
    schedule: "0 0 * 10 sat,sun"
    timezone: "Asia/Tokyo"
`

	cronhpa := &CronHorizontalPodAutoscaler{}
	err := yaml.Unmarshal([]byte(cronHPAManifest), cronhpa.ToCompatible())
	require.NoError(t, err)

	currentTime := time.Time{}
	_ = currentTime.UnmarshalText([]byte("2021-09-04T00:00:00+09:00"))
	cronhpa.Status.LastCronTimestamp = &metav1.Time{
		Time: currentTime,
	}

	// In-range weekday.
	_ = currentTime.UnmarshalText([]byte("2021-10-04T00:00:00+09:00")) // Mon
	patchName, err := cronhpa.GetCurrentPatchName(ctx, currentTime)
	require.NoError(t, err)
	if !assert.Equal(t, "weekday", patchName) {
		t.FailNow()
	}

	// In-range weekend.
	_ = currentTime.UnmarshalText([]byte("2021-10-02T00:00:00+09:00")) // Sat
	patchName, err = cronhpa.GetCurrentPatchName(ctx, currentTime)
	require.NoError(t, err)
	if !assert.Equal(t, "weekend", patchName) {
		t.FailNow()
	}

	// Out-range date
	_ = currentTime.UnmarshalText([]byte("2021-09-15T00:00:00+09:00")) // Wed
	patchName, err = cronhpa.GetCurrentPatchName(ctx, currentTime)
	require.NoError(t, err)
	if !assert.Empty(t, patchName) {
		t.FailNow()
	}

	// Out-range date with last patch name.
	_ = currentTime.UnmarshalText([]byte("2021-09-15T00:00:00+09:00")) // Wed
	cronhpa.Status.LastScheduledPatchName = "weekday"
	patchName, err = cronhpa.GetCurrentPatchName(ctx, currentTime)
	require.NoError(t, err)
	if !assert.Equal(t, "weekday", patchName) {
		t.FailNow()
	}

	// Lost scheduled patch
	cronhpa.Status.LastScheduledPatchName = "weekday"
	cronhpa.Spec.ScheduledPatches = []cronhpav1alpha1.CronHorizontalPodAutoscalerScheduledPatch{}
	_ = currentTime.UnmarshalText([]byte("2021-10-02T00:00:00+09:00")) // Sat
	patchName, err = cronhpa.GetCurrentPatchName(ctx, currentTime)
	require.NoError(t, err)
	if !assert.Empty(t, patchName) {
		t.FailNow()
	}

	// Without last timestamp
	cronhpa.Status.LastScheduledPatchName = "weekday"
	cronhpa.Status.LastCronTimestamp = nil
	_ = currentTime.UnmarshalText([]byte("2021-10-02T00:00:00+09:00")) // Sat
	patchName, err = cronhpa.GetCurrentPatchName(ctx, currentTime)
	require.NoError(t, err)
	if !assert.Empty(t, patchName) {
		t.FailNow()
	}
}

func TestCreateOrPatchHPA(t *testing.T) {
	ctx := context.TODO()

	cronHPAManifest := `
apiVersion: cron-hpa.ubie-oss.github.com/v1alpha1
kind: CronHorizontalPodAutoscaler
metadata:
  name: cron-hpa-sample
  namespace: default
spec:
  template:
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: cron-hpa-nginx
      minReplicas: 1
      maxReplicas: 10
      metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 50
  scheduledPatches:
  - name: weekday
    schedule: "0 0 * 10 mon-fri"
    timezone: "Asia/Tokyo"
    patch:
      minReplicas: 1
  - name: weekend
    schedule: "0 0 * 10 sat,sun"
    timezone: "Asia/Tokyo"
`

	cronhpa := &CronHorizontalPodAutoscaler{}
	err := yaml.Unmarshal([]byte(cronHPAManifest), cronhpa.ToCompatible())
	require.NoError(t, err)

	currentTime := time.Time{}
	_ = currentTime.UnmarshalText([]byte("2021-09-04T00:00:00+09:00"))

	fakeClient, err := test.NewFakeClient(ctx)
	require.NoError(t, err)
	reconciler := &CronHorizontalPodAutoscalerReconciler{
		Client:   fakeClient,
		Recorder: &test.FakeRecorder{},
	}

	// Create a CronHPA.
	err = reconciler.Create(ctx, cronhpa.ToCompatible())
	require.NoError(t, err)

	// Ensure no HPA.
	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err = reconciler.Get(ctx, types.NamespacedName{Namespace: "default", Name: "cron-hpa-sample"}, hpa)
	if !assert.True(t, errors.IsNotFound(err)) {
		t.FailNow()
	}

	// Create an HPA.
	err = cronhpa.CreateOrPatchHPA(ctx, "weekday", currentTime, reconciler)
	require.NoError(t, err)
	err = reconciler.Get(ctx, types.NamespacedName{Namespace: "default", Name: "cron-hpa-sample"}, hpa)
	require.NoError(t, err)
	if !assert.Equal(t, int32(1), *hpa.Spec.MinReplicas) {
		t.FailNow()
	}

	// Update an HPA.
	newMinReplicas := int32(2)
	cronhpa.Spec.ScheduledPatches[0].Patch.MinReplicas = &newMinReplicas
	err = cronhpa.CreateOrPatchHPA(ctx, "weekday", currentTime, reconciler)
	require.NoError(t, err)
	err = reconciler.Get(ctx, types.NamespacedName{Namespace: "default", Name: "cron-hpa-sample"}, hpa)
	require.NoError(t, err)
	if !assert.Equal(t, int32(2), *hpa.Spec.MinReplicas) {
		t.FailNow()
	}

	// Skip updating an HPA.
	err = reconciler.Get(ctx, types.NamespacedName{Namespace: "default", Name: "cron-hpa-sample"}, hpa)
	require.NoError(t, err)
	if !assert.Equal(t, int32(2), *hpa.Spec.MinReplicas) {
		t.FailNow()
	}
	hpa.Annotations = map[string]string{
		"cron-hpa.ubie-oss.github.com/skip": "true",
	}
	err = reconciler.Update(ctx, hpa)
	require.NoError(t, err)
	newMinReplicas = int32(3)
	cronhpa.Spec.ScheduledPatches[0].Patch.MinReplicas = &newMinReplicas
	err = cronhpa.CreateOrPatchHPA(ctx, "weekday", currentTime, reconciler)
	require.NoError(t, err)
	err = reconciler.Get(ctx, types.NamespacedName{Namespace: "default", Name: "cron-hpa-sample"}, hpa)
	require.NoError(t, err)
	if !assert.Equal(t, int32(2), *hpa.Spec.MinReplicas) {
		t.FailNow()
	}
}
