package materializationmodel

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLegacySchemaCannotSilentlyCarryNewStageFields(t *testing.T) {
	m := testModel()
	m.InputObservationProfiles = []InputObservationProfile{{TargetClass: "micro", Source: "fixture", Samples: 1, FixedSeconds: 3}}
	require.ErrorContains(t, m.Validate(), "schema version 2")
	m.SchemaVersion = 2
	require.NoError(t, m.Validate())
	m.SchemaVersion = 3
	require.Error(t, m.Validate())
}
