package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/containerd/nri/pkg/stub"
	"github.com/flavio/podlock/internal/nri"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// startPluginMode runs the NRI plugin mode.
func startPluginMode(ctx context.Context, client client.Client, logger *slog.Logger, pluginName, pluginIdx, logLevel string) {
	plugin := &nri.Plugin{
		LogLevel: logLevel,
		Logger:   logger,
		Client:   client,
	}
	var err error

	if err = copyBinaries(logger); err != nil {
		logger.ErrorContext(ctx, "failed to copy required binaries to host filesystem", slog.Any("err", err))
		os.Exit(1)
	}

	opts := []stub.Option{
		stub.WithOnClose(plugin.OnClose),
	}
	if pluginName != "" {
		opts = append(opts, stub.WithPluginName(pluginName))
	}
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	if plugin.Stub, err = stub.New(plugin, opts...); err != nil {
		logger.ErrorContext(ctx, "failed to create plugin stub", slog.Any("err", err))
		os.Exit(1)
	}

	if err = plugin.Stub.Run(ctx); err != nil {
		logger.ErrorContext(ctx, "plugin exited", slog.Any("err", err))
		os.Exit(1)
	}
}

// copyBinaries copies the required binaries to the host filesystem.
func copyBinaries(logger *slog.Logger) error {
	if err := nri.CopyFileIfDifferent(
		"/seal",
		filepath.Join("/host", nri.SealBinaryPathHost),
		logger,
	); err != nil {
		return fmt.Errorf("failed to copy seal binary to host: %w", err)
	}

	if err := nri.CopyFileIfDifferent(
		nri.SwapOciHookBinaryPathContainer,
		filepath.Join("/host", nri.SwapOciHookBinaryPathHost),
		logger,
	); err != nil {
		return fmt.Errorf("failed to copy swap-oci-hook binary to host: %w", err)
	}

	return nil
}
