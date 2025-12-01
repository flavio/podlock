package nri

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/flavio/podlock/pkg/constants"
)

// LabelNodeWithLandlockVersion adds a label to the specified node indicating the supported Landlock version.
func LabelNodeWithLandlockVersion(ctx context.Context, kubeClient client.Client, nodeName string, landlockVersion int) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node := &corev1.Node{}
		if err := kubeClient.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
			return fmt.Errorf("failed to get node %q: %w", nodeName, err)
		}

		if node.Labels == nil {
			node.Labels = map[string]string{}
		}
		node.Labels[constants.LandlockVersionNodeLabelKey] = strconv.Itoa(landlockVersion)

		return kubeClient.Update(ctx, node)
	})
	if err != nil {
		return fmt.Errorf("failed to label node %q with Landlock version: %w", nodeName, err)
	}
	return nil
}
