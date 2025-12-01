package nri

import (
	"context"
	"os"
	"testing"

	"log/slog"

	"github.com/containerd/nri/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/pkg/constants"
)

func TestCreateContainer_NoPodlockAnnotation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	plugin := &Plugin{
		LogLevel: "info",
		Logger:   logger,
		Client:   fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	pod := &api.PodSandbox{
		Name:      "testpod",
		Namespace: "default",
		Labels: map[string]string{
			"hello": "world",
		},
	}

	container := &api.Container{
		Name: "main",
	}

	adj, updates, err := plugin.CreateContainer(context.Background(), pod, container)
	require.NoError(t, err)
	assert.Nil(t, adj)
	assert.Nil(t, updates)
}

func TestCreateContainer_ProfileNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	plugin := &Plugin{
		LogLevel: "info",
		Logger:   logger,
		Client:   fake.NewClientBuilder().WithScheme(scheme).Build(), // No profiles added
	}

	// Pod with the special annotation pointing to a non-existent profile
	pod := &api.PodSandbox{
		Name:      "testpod",
		Namespace: "default",
		Labels: map[string]string{
			constants.PodProfileLabel: "missing-profile",
		},
	}

	container := &api.Container{
		Name: "main",
	}

	adj, updates, err := plugin.CreateContainer(context.Background(), pod, container)
	require.Error(t, err)
	assert.Nil(t, adj)
	assert.Nil(t, updates)
}
