/*
Copyright 2025.

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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const defaultNamespace = "default"

var _ = Describe("PodWatcher Controller", func() {
	Context("When reconciling Pods with watch label", func() {
		ctx := context.Background()

		It("should reconcile Pod with ecs.takutakahashi.dev/watch label", func() {
			podName := "test-pod-with-label"
			namespace := defaultNamespace

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels: map[string]string{
						"ecs.takutakahashi.dev/watch": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:alpine",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			podLookupKey := types.NamespacedName{Name: podName, Namespace: namespace}
			createdPod := &corev1.Pod{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, podLookupKey, createdPod)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdPod.Labels["ecs.takutakahashi.dev/watch"]).Should(Equal("true"))

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: podLookupKey,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, podLookupKey, &corev1.Pod{})
				return err != nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("should not reconcile Pod without watch label", func() {
			podName := "test-pod-without-label"
			namespace := defaultNamespace

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:alpine",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			podLookupKey := types.NamespacedName{Name: podName, Namespace: namespace}
			createdPod := &corev1.Pod{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, podLookupKey, createdPod)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdPod.Labels["ecs.takutakahashi.dev/watch"]).Should(BeEmpty())

			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())
		})

		It("should handle Pod deletion gracefully", func() {
			podName := "test-pod-deletion"
			namespace := defaultNamespace

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels: map[string]string{
						"ecs.takutakahashi.dev/watch": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:alpine",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			podLookupKey := types.NamespacedName{Name: podName, Namespace: namespace}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, podLookupKey, &corev1.Pod{})
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: podLookupKey,
			})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
