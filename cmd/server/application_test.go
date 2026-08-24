package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config/logger"
	"github.com/stretchr/testify/require"
)

func TestApplicationCompositionWithoutExternalKubernetes(t *testing.T) {
	t.Setenv("AKOFLOW_DATABASE_PATH", filepath.Join(t.TempDir(), "akoflow.db"))
	settings := config.Settings{
		HTTPAddress: ":0", DefaultNamespace: "akoflow",
		KubernetesCleanupEnabled: true,
	}
	application, err := newApplication(context.Background(), settings, logger.NewStdLogger())
	require.NoError(t, err)
	require.NotNil(t, application.database)
	require.NotNil(t, application.api)
	require.NotNil(t, application.eventLoop)
	require.Nil(t, application.historyCleaner)
	require.NoError(t, application.Close())
}
