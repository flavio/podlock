package nri

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"sigs.k8s.io/controller-runtime/pkg/client"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/pkg/constants"
)

type Plugin struct {
	LogLevel string
	Logger   *slog.Logger
	Stub     stub.Stub
	Client   client.Client
}

func (p *Plugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	if pod == nil {
		p.Logger.ErrorContext(ctx, "pod is nil")
		return nil, nil, errors.New("pod is nil")
	}

	profileName, enable := pod.GetLabels()[constants.PodProfileLabel]
	if !enable {
		p.Logger.DebugContext(ctx, "no podlock label found on pod, skipping mutation",
			slog.String("pod", pod.GetName()),
			slog.String("namespace", pod.GetNamespace()),
		)

		return nil, nil, nil
	}

	if ctr == nil {
		p.Logger.ErrorContext(ctx, "container is nil")
		return nil, nil, errors.New("container is nil")
	}

	// Fetch the LandlockProfile named "profileName" in the "default" namespace
	var profile podlockv1alpha1.LandlockProfile
	if err := p.Client.Get(ctx, client.ObjectKey{
		Namespace: pod.GetNamespace(),
		Name:      profileName,
	}, &profile); err != nil {
		p.Logger.ErrorContext(ctx, "failed to get LandlockProfile",
			slog.String("pod", pod.GetName()),
			slog.String("namespace", pod.GetNamespace()),
			slog.String("profile name", profileName),
			slog.Any("err", err),
		)
		return nil, nil, fmt.Errorf("failed to get LandlockProfile '%s': %w", profileName, err)
	}

	if profile.Spec.ProfilesByContainer == nil {
		p.Logger.InfoContext(ctx, "no profiles defined in LandlockProfile",
			slog.String("pod", pod.GetName()),
			slog.String("namespace", pod.GetNamespace()),
			slog.String("profile name", profileName),
		)
		return nil, nil, nil
	}

	profileByBinary, found := profile.Spec.ProfilesByContainer[ctr.GetName()]
	if !found {
		p.Logger.InfoContext(ctx, "no profile found for container",
			slog.String("pod", pod.GetName()),
			slog.String("namespace", pod.GetNamespace()),
			slog.String("profile name", profileName),
			slog.String("container name", ctr.GetName()),
		)
		return nil, nil, nil
	}

	if err := p.reserveSwappedBinaries(pod.GetId(), ctr.GetName(), profileByBinary); err != nil {
		p.Logger.ErrorContext(ctx, "failed to create container runtime dir",
			slog.String("pod", pod.GetName()),
			slog.String("namespace", pod.GetNamespace()),
			slog.String("container name", ctr.GetName()),
			slog.Any("err", err),
		)
		return nil, nil, err
	}

	if err := p.writeLandlockProfileToHostFilesystem(pod.GetId(), ctr.GetName(), profileByBinary); err != nil {
		p.Logger.ErrorContext(ctx, "failed to write landlock profile to host filesystem",
			slog.String("pod", pod.GetName()),
			slog.String("namespace", pod.GetNamespace()),
			slog.String("container name", ctr.GetName()),
			slog.Any("err", err),
		)
		return nil, nil, err
	}

	adjustment := createContainerAdjustment(pod.GetId(), ctr.GetName(), profileByBinary, p.LogLevel)

	p.Logger.InfoContext(ctx, "podlock annotation found, mutation requested",
		slog.String("namespace", pod.GetNamespace()),
		slog.String("pod", pod.GetName()),
		slog.String("container", ctr.GetName()),
		slog.String("adjustment", fmt.Sprintf("%+v", adjustment)),
	)

	return adjustment, nil, nil
}

func (p *Plugin) RemoveContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	p.Logger.InfoContext(ctx, "RemoveContainer called",
		slog.String("pod", pod.GetName()),
		slog.String("namespace", pod.GetNamespace()),
		slog.String("container", ctr.GetName()),
	)

	// Clean up swapped binaries
	podlockRuntimePodDir := filepath.Join(
		PodLockVarRunDir,
		pod.GetId(),
	)

	if err := os.RemoveAll(podlockRuntimePodDir); err != nil {
		p.Logger.ErrorContext(ctx, "failed to remove PodLock runtime dir",
			slog.String("pod", pod.GetName()),
			slog.String("namespace", pod.GetNamespace()),
			slog.String("container", ctr.GetName()),
			slog.String("swapped binaries dir", podlockRuntimePodDir),
			slog.Any("err", err),
		)
		return fmt.Errorf("failed to remove PodLock runtime dir '%s': %w", podlockRuntimePodDir, err)
	}

	p.Logger.InfoContext(ctx, "cleaned up swapped binaries",
		slog.String("pod", pod.GetName()),
		slog.String("namespace", pod.GetNamespace()),
		slog.String("container", ctr.GetName()),
		slog.String("swapped binaries dir", podlockRuntimePodDir),
	)
	return nil
}

func (p *Plugin) OnClose() {
	p.Logger.Info("Connection to the runtime lost, exiting...")
	os.Exit(1)
}
