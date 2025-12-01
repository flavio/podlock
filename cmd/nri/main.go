package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/internal/cmdutil"
)

func setupLogger(logLevel string) *slog.Logger {
	slogLevel, err := cmdutil.ParseLogLevel(logLevel)
	if err != nil {
		//nolint:sloglint // Use the global logger since the logger is not yet initialized
		slog.Error(
			"error initializing the logger",
			"error",
			err,
		)
		os.Exit(1)
	}
	opts := slog.HandlerOptions{
		Level: slogLevel,
	}
	slogHandler := slog.NewJSONHandler(os.Stdout, &opts)
	slogger := slog.New(slogHandler).With("component", "nri")

	logger := logr.FromSlogHandler(slogHandler).WithValues("component", "nri")
	ctrl.SetLogger(logger)

	return slogger
}

func setupKubeClient(ctx context.Context, logger slog.Logger) (client.Client, error) {
	scheme := runtime.NewScheme()
	err := podlockv1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to add podlockv1alpha1 to scheme: %w", err)
	}

	// register core types:
	if err = v1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add corev1 to scheme: %w", err)
	}

	restConfig := config.GetConfigOrDie()

	// Create and start the cache
	k8sCache, err := cache.New(restConfig, cache.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	go func() {
		if err = k8sCache.Start(ctx); err != nil {
			logger.Error("failed to start cache", slog.Any("err", err))
			os.Exit(1)
		}
	}()
	// Wait for cache to sync
	if synced := k8sCache.WaitForCacheSync(ctx); !synced {
		return nil, errors.New("cache did not sync")
	}

	// Use the cache as the kubeClient's reader
	kubeClient, err := client.New(restConfig, client.Options{
		Scheme: scheme,
		Cache:  &client.CacheOptions{Reader: k8sCache},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create kube client: %w", err)
	}

	return kubeClient, nil
}

func main() {
	var (
		pluginName string
		pluginIdx  string
		err        error
		logLevel   string
		initMode   bool
	)

	flag.StringVar(&pluginName, "name", "", "plugin name to register to NRI")
	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.StringVar(&logLevel, "log-level", slog.LevelInfo.String(), "Log level.")
	flag.BoolVar(&initMode, "init-mode", false, "Run in init mode to detect kernel features.")
	flag.Parse()

	logger := setupLogger(logLevel)
	logger.Info("Starting NRI plugin")

	ctx := context.Background()

	kubeClient, err := setupKubeClient(ctx, *logger)
	if err != nil {
		logger.Error("failed to setup Kubernetes client", slog.Any("err", err))
		os.Exit(1)
	}

	if initMode {
		startInitMode(ctx, kubeClient, logger)
	} else {
		startPluginMode(ctx, kubeClient, logger, pluginName, pluginIdx, logLevel)
	}
}
