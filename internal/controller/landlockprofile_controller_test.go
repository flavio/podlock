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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/pkg/constants"
)

var _ = Describe("LandlockProfile Controller", func() {
	Context("When reconciling a resource", func() {
		const profileName = "test-resource"
		const testNamespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      profileName,
			Namespace: testNamespace,
		}
		profile := &v1alpha1.LandlockProfile{}

		BeforeEach(func() {
			By("Creating a new LandlockProfile that is ")
			err := k8sClient.Get(ctx, typeNamespacedName, profile)
			if err != nil && errors.IsNotFound(err) {
				resource :=
					&v1alpha1.LandlockProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name:       profileName,
							Namespace:  testNamespace,
							Finalizers: []string{v1alpha1.LandlockProfileFinalizer},
						},
						Spec: v1alpha1.LandlockProfileSpec{
							ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
								"main": {
									"/usr/sbin/nginx": {
										ReadOnly: []string{"/etc/nginx", "/var/www/html"},
									},
								},
							},
						},
					}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha1.LandlockProfile{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if errors.IsNotFound(err) {
				// Resource already deleted, nothing to clean up
				return
			}
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance LandlockProfile")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("Should delete a LandlockProfile that is not referenced by any Pod", func() {
			By("Deleting the created LandlockProfile")
			profile = &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      profileName,
					Namespace: testNamespace,
				},
			}
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &LandlockProfileReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the LandlockProfile has been deleted")
			err = k8sClient.Get(ctx, typeNamespacedName, profile)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("Should not delete a LandlockProfile that is referenced by a Pod", func() {
			By("Associating the LandlockProfile with a Pod")
			podName := "test-pod"
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
					Labels: map[string]string{
						constants.PodProfileLabel: profileName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "pause",
							Image: "registry.k8s.io/pause",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			By("Deleting the referenced LandlockProfile")
			profile = &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      profileName,
					Namespace: testNamespace,
				},
			}
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())

			By("Reconciling the deleted LandlockProfile")
			controllerReconciler := &LandlockProfileReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the LandlockProfile has not been deleted")
			err = k8sClient.Get(ctx, typeNamespacedName, profile)
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the created Pod")
			Expect(k8sClient.Delete(ctx, pod)).To(Succeed())

			By("Reconciling the LandlockProfile deletion again")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the LandlockProfile has been deleted after Pod removal")
			err = k8sClient.Get(ctx, typeNamespacedName, profile)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

})
