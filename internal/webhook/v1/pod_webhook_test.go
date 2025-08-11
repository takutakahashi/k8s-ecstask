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

package v1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Pod Webhook", func() {
	var (
		obj       *corev1.Pod
		oldObj    *corev1.Pod
		defaulter PodCustomDefaulter
	)

	BeforeEach(func() {
		obj = &corev1.Pod{}
		oldObj = &corev1.Pod{}
		defaulter = PodCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		// TODO (user): Add any setup logic common to all tests
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("When creating Pod under Defaulting Webhook", func() {
		It("Should block Pod with watch label", func() {
			By("creating a Pod with watch label")
			obj = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
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

			By("calling the Default method")
			err := defaulter.Default(context.Background(), obj)

			By("checking that the Pod creation is blocked")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pods with label 'ecs.takutakahashi.dev/watch' are not allowed"))
		})

		It("Should also block Pod with watch label and additional annotations", func() {
			By("creating a Pod with watch label and other annotations")
			obj = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "blocked-pod",
					Namespace: "default",
					Labels: map[string]string{
						"ecs.takutakahashi.dev/watch": "true",
						"app":                         "test",
					},
					Annotations: map[string]string{
						"some-annotation": "value",
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

			By("calling the Default method")
			err := defaulter.Default(context.Background(), obj)

			By("checking that the Pod creation is blocked")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pods with label 'ecs.takutakahashi.dev/watch' are not allowed"))
		})

		It("Should ignore Pod without watch label", func() {
			By("creating a Pod without watch label")
			obj = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "normal-pod",
					Namespace: "default",
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

			By("calling the Default method")
			err := defaulter.Default(context.Background(), obj)

			By("checking that the Pod is allowed without modifications")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Annotations).To(BeNil())
		})
	})

})
