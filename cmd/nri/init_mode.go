package main

import (
	"context"
	"log/slog"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/flavio/podlock/internal/nri"
)

const NodeNameEvar = "NODE_NAME"

func startInitMode(ctx context.Context, kubeClient client.Client, logger *slog.Logger) {
	logger.InfoContext(ctx, "Running in init mode to detect kernel features")

	nodeName := os.Getenv(NodeNameEvar)
	if nodeName == "" {
		logger.ErrorContext(ctx, "NODE_NAME environment variable is not set")
		os.Exit(1)
	}

	version := nri.DetectLandlockVersion(logger)

	if err := nri.LabelNodeWithLandlockVersion(ctx, kubeClient, nodeName, version); err != nil {
		logger.ErrorContext(ctx, "Failed to update node with retry", slog.String("node", nodeName), slog.Any("error", err))
		os.Exit(1)
	}

	logger.InfoContext(ctx, "Successfully labeled node", slog.String("node", nodeName))
}
