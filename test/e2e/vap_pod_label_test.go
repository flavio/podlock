package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/pkg/constants"
)

func TestValidatingAdmissionPolicyPodProfileLabel(t *testing.T) {
	releaseName := "podlock"
	chartPath := "../../charts/podlock"
	testNamespace := "default"
	profileName := "pause-profile"
	vapRejectionMessage := "The label 'podlock.kubewarden.io/profile' is immutable. You cannot add, remove, or change its value."

	f := features.New("Test ValidatingAdmissionPolicy for Pod profile label").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			manager := helm.New(cfg.KubeconfigFile())
			err := manager.RunInstall(helm.WithName(releaseName),
				helm.WithNamespace(cfg.Namespace()),
				helm.WithChart(chartPath),
				helm.WithWait(),
				helm.WithArgs("--set", "vap.enabled=true",
					"--set", "controller.image.tag=latest",
					"--set", "nri.image.tag=latest",
					"--set", "nri.logLevel=debug"),
				helm.WithTimeout("6m"))

			require.NoError(t, err, "podlock helm chart with VAP enabled is not installed correctly")

			return ctx
		}).
		Assess("Creating a LandlockProfile for pause container", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			landlockProfile := &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      profileName,
					Namespace: testNamespace,
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"pause": {
							"/pause": {},
						},
					},
				},
			}

			err := cfg.Client().Resources().Create(context.Background(), landlockProfile)
			require.NoError(t, err, "failed to create LandlockProfile")

			return ctx
		}).
		Assess("Adding podlock.kubewarden.io/profile label should be denied", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "test-pod-add-label"

			// Create pod without label first
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "pause",
						Image: "registry.k8s.io/pause",
					}},
				},
			}
			err := cfg.Client().Resources().Create(ctx, pod)
			require.NoError(t, err, "failed to create pod without label")

			// Wait for pod to be running to reduce status update conflicts
			err = wait.For(conditions.New(cfg.Client().Resources()).PodRunning(pod), wait.WithTimeout(30*time.Second))
			require.NoError(t, err, "pod failed to reach running state")

			// Try to add label - should be denied by VAP
			var updateErr error
			err = wait.For(func(ctx context.Context) (bool, error) {
				freshPod := &corev1.Pod{}
				if err := cfg.Client().Resources().Get(ctx, podName, testNamespace, freshPod); err != nil {
					return false, err
				}
				freshPod.Labels = map[string]string{constants.PodProfileLabel: profileName}

				updateErr = cfg.Client().Resources().Update(ctx, freshPod)
				if updateErr == nil {
					// Update succeeded - this is unexpected
					return true, nil
				}
				if strings.Contains(updateErr.Error(), "the object has been modified") {
					// Conflict error - retry
					return false, nil
				}
				// Got a non-conflict error (should be VAP rejection) - done
				return true, nil
			}, wait.WithTimeout(30*time.Second), wait.WithInterval(500*time.Millisecond))
			require.NoError(t, err, "failed waiting for update attempt")

			require.Error(t, updateErr, "expected error when adding podlock profile label")
			assert.Contains(t, updateErr.Error(), vapRejectionMessage, "error message should match expected admission denial")

			// Clean up
			err = cfg.Client().Resources().Delete(ctx, pod)
			assert.NoError(t, err, "failed to cleanup pod")

			return ctx
		}).
		Assess("Removing podlock.kubewarden.io/profile label should be denied", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "test-pod-remove-label"

			// Create pod with label
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
					Labels: map[string]string{
						constants.PodProfileLabel: profileName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "pause",
						Image: "registry.k8s.io/pause",
					}},
				},
			}
			err := cfg.Client().Resources().Create(ctx, pod)
			require.NoError(t, err, "failed to create pod with label")

			// Wait for pod to be running to reduce status update conflicts
			err = wait.For(conditions.New(cfg.Client().Resources()).PodRunning(pod), wait.WithTimeout(30*time.Second))
			require.NoError(t, err, "pod failed to reach running state")

			// Try to remove label - should be denied by VAP
			var updateErr error
			err = wait.For(func(ctx context.Context) (bool, error) {
				freshPod := &corev1.Pod{}
				if err := cfg.Client().Resources().Get(ctx, podName, testNamespace, freshPod); err != nil {
					return false, err
				}

				updatedPod := freshPod.DeepCopy()
				delete(updatedPod.Labels, constants.PodProfileLabel)

				updateErr = cfg.Client().Resources().Update(ctx, updatedPod)
				if updateErr == nil {
					// Update succeeded - this is unexpected
					return true, nil
				}
				if strings.Contains(updateErr.Error(), "the object has been modified") {
					// Conflict error - retry
					return false, nil
				}
				// Got a non-conflict error (should be VAP rejection) - done
				return true, nil
			}, wait.WithTimeout(30*time.Second), wait.WithInterval(500*time.Millisecond))
			require.NoError(t, err, "failed waiting for update attempt")

			require.Error(t, updateErr, "expected error when removing podlock profile label")
			assert.Contains(t, updateErr.Error(), vapRejectionMessage, "error message should match expected admission denial")

			// Clean up
			pod.Labels = map[string]string{constants.PodProfileLabel: "test-profile"} // restore label for deletion
			err = cfg.Client().Resources().Delete(ctx, pod)
			assert.NoError(t, err, "failed to cleanup pod")

			return ctx
		}).
		Assess("Changing podlock.kubewarden.io/profile label value should be denied", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "test-pod-change-label"

			// Create pod with label
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
					Labels: map[string]string{
						constants.PodProfileLabel: profileName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "pause",
						Image: "registry.k8s.io/pause",
					}},
				},
			}
			err := cfg.Client().Resources().Create(ctx, pod)
			require.NoError(t, err, "failed to create pod with original label")

			// Wait for pod to be running to reduce status update conflicts
			err = wait.For(conditions.New(cfg.Client().Resources()).PodRunning(pod), wait.WithTimeout(30*time.Second))
			require.NoError(t, err, "pod failed to reach running state")

			// Try to change label value - should be denied by VAP
			var updateErr error
			err = wait.For(func(ctx context.Context) (bool, error) {
				freshPod := &corev1.Pod{}
				if err := cfg.Client().Resources().Get(ctx, podName, testNamespace, freshPod); err != nil {
					return false, err
				}

				updatedPod := freshPod.DeepCopy()
				updatedPod.Labels[constants.PodProfileLabel] = "new-profile"

				updateErr = cfg.Client().Resources().Update(ctx, updatedPod)
				if updateErr == nil {
					// Update succeeded - this is unexpected
					return true, nil
				}
				if strings.Contains(updateErr.Error(), "the object has been modified") {
					// Conflict error - retry
					return false, nil
				}
				// Got a non-conflict error (should be VAP rejection) - done
				return true, nil
			}, wait.WithTimeout(30*time.Second), wait.WithInterval(500*time.Millisecond))
			require.NoError(t, err, "failed waiting for update attempt")

			require.Error(t, updateErr, "expected error when changing podlock profile label value")
			assert.Contains(t, updateErr.Error(), vapRejectionMessage, "error message should match expected admission denial")

			// Clean up
			err = cfg.Client().Resources().Delete(ctx, pod)
			assert.NoError(t, err, "failed to cleanup pod")

			return ctx
		}).
		Assess("Updating other Pod fields should be allowed when profile label exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "test-pod-update-fields"

			// Create pod with label
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
					Labels: map[string]string{
						constants.PodProfileLabel: profileName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "pause",
						Image: "registry.k8s.io/pause",
					}},
				},
			}
			err := cfg.Client().Resources().Create(ctx, pod)
			require.NoError(t, err, "failed to create pod with label")

			// Wait for pod to be running to reduce status update conflicts
			err = wait.For(conditions.New(cfg.Client().Resources()).PodRunning(pod), wait.WithTimeout(30*time.Second))
			require.NoError(t, err, "pod failed to reach running state")

			// Update other fields - should be allowed
			err = wait.For(func(ctx context.Context) (bool, error) {
				freshPod := &corev1.Pod{}
				if err := cfg.Client().Resources().Get(ctx, podName, testNamespace, freshPod); err != nil {
					return false, err
				}

				updatedPod := freshPod.DeepCopy()
				updatedPod.Labels["other-label"] = "other-value"
				if updatedPod.Annotations == nil {
					updatedPod.Annotations = make(map[string]string)
				}
				updatedPod.Annotations["updated"] = "true"

				updateErr := cfg.Client().Resources().Update(ctx, updatedPod)
				if updateErr == nil {
					// Success!
					return true, nil
				}
				if strings.Contains(updateErr.Error(), "the object has been modified") {
					// Conflict error - retry
					return false, nil
				}
				// Got a non-conflict error - this is unexpected, fail the test
				return false, updateErr
			}, wait.WithTimeout(30*time.Second), wait.WithInterval(500*time.Millisecond))
			require.NoError(t, err, "expected no error when updating other pod fields")

			// Clean up
			err = cfg.Client().Resources().Delete(ctx, pod)
			assert.NoError(t, err, "failed to cleanup pod")

			return ctx
		}).
		Assess("Updating other Pod fields should be allowed when profile label doesn't exist", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "test-pod-no-profile-update"

			// Create pod without profile label
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "pause",
						Image: "registry.k8s.io/pause",
					}},
				},
			}
			err := cfg.Client().Resources().Create(ctx, pod)
			require.NoError(t, err, "failed to create pod without profile label")

			// Wait for pod to be running to reduce status update conflicts
			err = wait.For(conditions.New(cfg.Client().Resources()).PodRunning(pod), wait.WithTimeout(30*time.Second))
			require.NoError(t, err, "pod failed to reach running state")

			// Update other fields - should be allowed
			err = wait.For(func(ctx context.Context) (bool, error) {
				freshPod := &corev1.Pod{}
				if err := cfg.Client().Resources().Get(ctx, podName, testNamespace, freshPod); err != nil {
					return false, err
				}

				updatedPod := freshPod.DeepCopy()
				updatedPod.Labels = map[string]string{"other-label": "other-value"}
				if updatedPod.Annotations == nil {
					updatedPod.Annotations = make(map[string]string)
				}
				updatedPod.Annotations["updated"] = "true"

				updateErr := cfg.Client().Resources().Update(ctx, updatedPod)
				if updateErr == nil {
					// Success!
					return true, nil
				}
				if strings.Contains(updateErr.Error(), "the object has been modified") {
					// Conflict error - retry
					return false, nil
				}
				// Got a non-conflict error - this is unexpected, fail the test
				return false, updateErr
			}, wait.WithTimeout(30*time.Second), wait.WithInterval(500*time.Millisecond))
			require.NoError(t, err, "expected no error when updating pod without profile label")

			// Clean up
			err = cfg.Client().Resources().Delete(ctx, pod)
			assert.NoError(t, err, "failed to cleanup pod")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			manager := helm.New(cfg.KubeconfigFile())
			err := manager.RunUninstall(
				helm.WithName(releaseName),
				helm.WithNamespace(cfg.Namespace()),
			)
			assert.NoError(t, err, "podlock helm chart is not deleted correctly")
			return ctx
		})

	testenv.Test(t, f.Feature())
}
