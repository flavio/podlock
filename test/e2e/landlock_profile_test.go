package e2e

import (
	"context"
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

func TestLandlockProfileCreation(t *testing.T) {
	releaseName := "podlock"
	chartPath := "../../charts/podlock"
	landlockProfileName := "test-landlock-profile"
	testNamespace := "default"

	f := features.New("Deploy PodLock").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			manager := helm.New(cfg.KubeconfigFile())
			err := manager.RunInstall(helm.WithName(releaseName),
				helm.WithNamespace(cfg.Namespace()),
				helm.WithChart(chartPath),
				helm.WithWait(),
				helm.WithArgs("--set", "controller.image.tag=latest",
					"--set", "nri.image.tag=latest",
					"--set", "nri.logLevel=debug",
				),
				helm.WithTimeout("6m"))

			require.NoError(t, err, "podlock helm chart is not installed correctly")

			err = v1alpha1.AddToScheme(cfg.Client().Resources(cfg.Namespace()).GetScheme())
			require.NoError(t, err)

			return ctx
		}).
		Assess("Create a LandlockProfile", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create the LandlockProfile
			landlockProfile := &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      landlockProfileName,
					Namespace: testNamespace,
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

			err := cfg.Client().Resources().Create(ctx, landlockProfile)
			require.NoError(t, err, "failed to create LandlockProfile")

			return ctx
		}).
		Assess("Verify the finalizer is set", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Get the profile and verify the finalizer is present
			landlockProfile := &v1alpha1.LandlockProfile{}
			err := cfg.Client().Resources().Get(ctx, landlockProfileName, testNamespace, landlockProfile)
			require.NoError(t, err, "failed to get LandlockProfile")

			assert.Contains(t, landlockProfile.Finalizers, v1alpha1.LandlockProfileFinalizer, "LandlockProfile finalizer is not set")

			return ctx
		}).
		Assess("Verify a not referenced profile can be deleted", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Delete the profile
			landlockProfile := &v1alpha1.LandlockProfile{}
			err := cfg.Client().Resources().Get(ctx, landlockProfileName, testNamespace, landlockProfile)
			require.NoError(t, err, "failed to get LandlockProfile for deletion")

			err = cfg.Client().Resources().Delete(ctx, landlockProfile)
			require.NoError(t, err, "failed to delete LandlockProfile")

			// Wait for the profile to be deleted
			err = wait.For(
				conditions.New(cfg.Client().Resources()).ResourceDeleted(landlockProfile),
				wait.WithTimeout(time.Minute*2),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(t, err, "LandlockProfile was not deleted within timeout")

			return ctx
		}).
		Assess("Verify a referenced profile cannot be deleted", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			referencedProfileName := "referenced-landlock-profile"
			podName := "test-pod-with-profile"

			// Create a new LandlockProfile
			referencedProfile := &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      referencedProfileName,
					Namespace: testNamespace,
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			}
			err := cfg.Client().Resources().Create(ctx, referencedProfile)
			require.NoError(t, err, "failed to create referenced LandlockProfile")

			// Create a Pod that references this profile
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: testNamespace,
					Labels: map[string]string{
						constants.PodProfileLabel: referencedProfileName,
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
			err = cfg.Client().Resources().Create(ctx, pod)
			require.NoError(t, err, "failed to create Pod")

			// Try to delete the profile
			err = cfg.Client().Resources().Delete(ctx, referencedProfile)
			require.NoError(t, err, "failed to issue delete request for LandlockProfile")

			// Wait a reasonable amount of time and verify the profile still exists
			time.Sleep(30 * time.Second)

			// Verify the profile still exists (should not be deleted due to finalizer)
			profileCheck := &v1alpha1.LandlockProfile{}
			err = cfg.Client().Resources().Get(ctx, referencedProfileName, testNamespace, profileCheck)
			require.NoError(t, err, "LandlockProfile should still exist while referenced by Pod")
			assert.NotNil(t, profileCheck.DeletionTimestamp, "DeletionTimestamp should be set")
			assert.Contains(t, profileCheck.Finalizers, v1alpha1.LandlockProfileFinalizer, "Finalizer should still be present")

			// Clean up: delete the pod
			err = cfg.Client().Resources().Delete(ctx, pod)
			require.NoError(t, err, "failed to delete Pod")

			// Wait for the pod to be deleted
			err = wait.For(
				conditions.New(cfg.Client().Resources()).ResourceDeleted(pod),
				wait.WithTimeout(time.Minute*2),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(t, err, "Pod was not deleted within timeout")

			// Now the profile should be deleted automatically
			err = wait.For(
				conditions.New(cfg.Client().Resources()).ResourceDeleted(referencedProfile),
				wait.WithTimeout(time.Minute*2),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(t, err, "LandlockProfile should be deleted after Pod is removed")

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
