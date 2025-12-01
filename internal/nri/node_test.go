package nri

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/flavio/podlock/pkg/constants"
)

func TestLabelNodeWithLandlockVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name            string
		existingLabels  map[string]string
		landlockVersion int
		expectLabels    map[string]string
	}{
		{
			name:            "add label to node with no labels",
			existingLabels:  nil,
			landlockVersion: 3,
			expectLabels:    map[string]string{constants.LandlockVersionNodeLabelKey: "3"},
		},
		{
			name:            "overwrite existing landlock label",
			existingLabels:  map[string]string{constants.LandlockVersionNodeLabelKey: "1"},
			landlockVersion: 5,
			expectLabels:    map[string]string{constants.LandlockVersionNodeLabelKey: "5"},
		},
		{
			name:            "preserve other labels",
			existingLabels:  map[string]string{"foo": "bar"},
			landlockVersion: 2,
			expectLabels:    map[string]string{"foo": "bar", constants.LandlockVersionNodeLabelKey: "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{}
			node.Name = "testnode"
			if tt.existingLabels != nil {
				// Copy to avoid mutation issues
				node.Labels = make(map[string]string, len(tt.existingLabels))
				maps.Copy(node.Labels, tt.existingLabels)
			}

			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()
			err := LabelNodeWithLandlockVersion(context.Background(), c, "testnode", tt.landlockVersion)
			require.NoError(t, err)

			updated := &corev1.Node{}
			err = c.Get(context.Background(), client.ObjectKey{Name: "testnode"}, updated)
			require.NoError(t, err)
			assert.Equal(t, tt.expectLabels, updated.Labels)
		})
	}
}
