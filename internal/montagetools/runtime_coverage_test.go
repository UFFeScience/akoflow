package montagetools

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCoverageDoesNotTreatRecoveredCountersAsIndependentActivities(t *testing.T) {
	_, training := stageFixture()
	obj(training["run"])["completedActivityCount"] = 58
	h := clone(rows(training["handles"])[0])
	h["status"] = "failed"
	training["handles"] = append(arr(training["handles"]), h)
	r := runtimeReferenceCoverage([]Object{training})
	row := rows(r["references"])[0]
	require.Equal(t, 2, row["distinctActivities"])
	require.Equal(t, false, row["eligibleCleanWholeRunReference"])
	require.Equal(t, 1, rows(row["classes"])[0]["singleHandleActivities"])
}
