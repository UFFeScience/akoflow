package materializationmodel

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"math"
	"os"
	"testing"
)

func TestPublicationServiceUsesWorkspaceAndInheritedInputs(t *testing.T) {
	m := testModel()
	m.PublicationProfiles = []PublicationProfile{{TargetClass: "micro", Source: "measured-fixture", Samples: 1, ScanSecondsPerWorkspaceByte: .02, CleanupSecondsPerInputByte: .01}}
	seconds, err := m.PublicationSeconds("cloud-a", 100, 50)
	require.NoError(t, err)
	require.InDelta(t, 2.5, seconds, 1e-9)
	_, err = m.PublicationSeconds("cloud", -1, 0)
	require.Error(t, err)
	m.PublicationProfiles[0].Source = ""
	require.Error(t, m.Validate())
}

func TestFixedStageServiceIsCommonAndClassScoped(t *testing.T) {
	m := testModel()
	m.InputObservationProfiles = []InputObservationProfile{{TargetClass: "micro", Source: "fixture", Samples: 1, FixedSeconds: 3, SecondsPerUniqueByte: .01}}
	m.PublicationProfiles = []PublicationProfile{{TargetClass: "micro", Source: "fixture", Samples: 1, FixedSeconds: 5, ScanSecondsPerWorkspaceByte: .02}}
	p, err := m.PreparationSeconds("cloud-a", 0, 0)
	require.NoError(t, err)
	require.Equal(t, 3.0, p)
	p, err = m.PublicationSeconds("cloud-a", 100, 0)
	require.NoError(t, err)
	require.Equal(t, 7.0, p)
	m.InputObservationProfiles[0].FixedSeconds = -1
	require.Error(t, m.Validate())
}

func TestFrozenMontageMeasuredModelDecodesAndRejectsUnobservedPair(t *testing.T) {
	payload, err := os.ReadFile("../../../experiments/montage-micro-hpc-e2/materialization-model-v2.json")
	require.NoError(t, err)
	var model Model
	require.NoError(t, json.Unmarshal(payload, &model))
	require.NoError(t, model.Validate())
	route, err := model.Route("plafrim-execution-ssh-node-diablo03", "montage-gcp-fog-e2-micro-01", 200_000_000)
	require.NoError(t, err)
	require.Len(t, route.Stages, 2)
	require.Equal(t, "montage-relay-controller", route.Stages[0].TargetResourceID)
	require.Equal(t, route.NetworkBytes, route.RelayNetworkBytes)
	_, err = model.Route("plafrim-execution-ssh-node-diablo03", "montage-gcp-e2-medium-1", 200_000_000)
	require.ErrorContains(t, err, "no evidenced")
}

func testModel() Model {
	return Model{SchemaVersion: 1, Source: "completed-baseline", RelayResourceID: "relay", Resources: map[string]Resource{
		"cloud-a": {Class: "micro", Domain: "cloud"}, "cloud-b": {Class: "micro", Domain: "cloud"},
		"hpc-a": {Class: "hpc", Domain: "hpc"}, "hpc-b": {Class: "hpc", Domain: "hpc"}, "relay": {Class: "relay", Domain: "relay"},
	}, Profiles: []Profile{
		{Strategy: "direct-runtime", SourceClass: "micro", TargetClass: "micro", Source: "baseline", Samples: 3, ArchiveToWorkspaceRatio: .5, ProbeSeconds: 1, SnapshotSecondsPerByte: .01, UploadArchiveBytesPerSecond: 25, ReceiverSecondsPerByte: .02},
		{Strategy: "gateway", SourceClass: "micro", TargetClass: "hpc", Source: "baseline", Samples: 3, ArchiveToWorkspaceRatio: .5, ProbeSeconds: 1, IngressArchiveBytesPerSecond: 50, ValidationSecondsPerByte: .01, UploadArchiveBytesPerSecond: 25, ReceiverSecondsPerByte: .02, IngressIncludesSourcePreparation: true},
		{Strategy: "gateway", SourceClass: "hpc", TargetClass: "micro", Source: "baseline", Samples: 3, ArchiveToWorkspaceRatio: .5, ProbeSeconds: 1, IngressArchiveBytesPerSecond: 50, ValidationSecondsPerByte: .01, UploadArchiveBytesPerSecond: 25, ReceiverSecondsPerByte: .02, IngressIncludesSourcePreparation: true},
		{Strategy: "shared-storage", SourceClass: "hpc", TargetClass: "hpc", Source: "storage-probe", Samples: 3, SnapshotSecondsPerByte: .01, ReceiverSecondsPerByte: .02},
	}, MergeProfiles: []MergeProfile{{TargetClass: "micro", Source: "baseline", Samples: 3, CheckSecondsPerInputByte: .01, CopySecondsPerUniqueByte: .02, VerifySecondsPerUniqueByte: .03}}}
}

func TestInputObservationIsChargedOnceOnUniqueInputs(t *testing.T) {
	m := testModel()
	m.InputObservationProfiles = []InputObservationProfile{{TargetClass: "micro", Source: "measured-snapshot-service", Samples: 3, SecondsPerUniqueByte: .5}}
	merge, err := m.MergeSeconds("cloud-a", 200, 100)
	require.NoError(t, err)
	preparation, err := m.PreparationSeconds("cloud-a", 200, 100)
	require.NoError(t, err)
	require.Equal(t, merge+50, preparation)
	root, err := m.PreparationSeconds("cloud-a", 0, 0)
	require.NoError(t, err)
	require.Zero(t, root)
	m.InputObservationProfiles[0].Source = ""
	require.Error(t, m.Validate())
}

func TestSeparatedIngressUsesIndependentPreparationWithoutExtraNetworkBytes(t *testing.T) {
	m := testModel()
	p := &m.Profiles[2]
	p.IngressIncludesSourcePreparation = false
	p.SeparatedIngressServiceSource = "queue-free-independent-snapshot-and-ingress"
	p.SnapshotSecondsPerByte = .03
	route, err := m.Route("hpc-a", "cloud-a", 100)
	require.NoError(t, err)
	require.Len(t, route.Stages, 2)
	require.Equal(t, "relay-source-preparation-and-ingress", route.Stages[0].Name)
	require.Equal(t, 4.0, route.Stages[0].BeforeSeconds)
	require.Equal(t, 50.0, route.Stages[0].EffectiveBytesPerSecond)
	require.EqualValues(t, 100, route.NetworkBytes)
	p.IngressIncludesSourcePreparation = true
	require.ErrorContains(t, m.Validate(), "cannot also charge")
}

func TestBulkServiceGroupsFollowControlTransportNotRelayHost(t *testing.T) {
	m := testModel()
	hpc := m.Resources["hpc-a"]
	hpc.BulkServiceGroupID, hpc.BulkServiceSlots, hpc.BulkServiceSource = "hpc-connection", 3, "provider-config"
	m.Resources["hpc-a"] = hpc
	cloud := m.Resources["cloud-a"]
	cloud.BulkServiceGroupID, cloud.BulkServiceSlots, cloud.BulkServiceSource = "cloud-instance", 3, "provider-config"
	m.Resources["cloud-a"] = cloud
	require.NoError(t, m.Validate())
	route, err := m.Route("hpc-a", "cloud-a", 100)
	require.NoError(t, err)
	require.Equal(t, "hpc-connection", route.Stages[0].ServiceGroupID)
	require.Equal(t, "cloud-instance", route.Stages[1].ServiceGroupID)
	require.Equal(t, 3, route.Stages[0].ServiceSlots)
	require.Equal(t, 3, route.Stages[1].ServiceSlots)
	other := m.Resources["hpc-b"]
	other.BulkServiceGroupID, other.BulkServiceSlots, other.BulkServiceSource = "hpc-connection", 2, "provider-config"
	m.Resources["hpc-b"] = other
	require.Error(t, m.Validate(), "one transport cannot have conflicting slot capacities")
}

func TestPeerReceiverUsesDestinationServicePoolWithoutExtraNetworkCopy(t *testing.T) {
	m := testModel()
	for _, id := range []string{"cloud-a", "cloud-b"} {
		r := m.Resources[id]
		r.BulkServiceGroupID, r.BulkServiceSlots, r.BulkServiceSource = id+"-session", 3, "provider-config"
		m.Resources[id] = r
	}
	route, err := m.Route("cloud-a", "cloud-b", 100)
	require.NoError(t, err)
	require.Len(t, route.Stages, 2)
	require.Equal(t, "cloud-a-session", route.Stages[0].ServiceGroupID)
	require.Equal(t, "cloud-b-session", route.Stages[1].ServiceGroupID)
	require.EqualValues(t, 50, route.Stages[0].Bytes)
	require.Zero(t, route.Stages[1].Bytes)
	require.Zero(t, route.Stages[0].AfterSeconds)
	require.Equal(t, 2.0, route.Stages[1].BeforeSeconds)
	require.EqualValues(t, 50, route.NetworkBytes)
	require.Zero(t, route.RelayNetworkBytes)
}

func TestRelayIsMandatoryInBothDirectionsAndHasTwoSerialStages(t *testing.T) {
	m := testModel()
	for _, pair := range [][2]string{{"cloud-a", "hpc-a"}, {"hpc-a", "cloud-a"}} {
		route, err := m.Route(pair[0], pair[1], 100)
		require.NoError(t, err)
		require.Equal(t, "gateway", route.Strategy)
		require.Len(t, route.Stages, 2)
		require.Equal(t, pair[0], route.Stages[0].SourceResourceID)
		require.Equal(t, "relay", route.Stages[0].TargetResourceID)
		require.Equal(t, "relay", route.Stages[1].SourceResourceID)
		require.Equal(t, pair[1], route.Stages[1].TargetResourceID)
		require.EqualValues(t, 100, route.NetworkBytes)
		require.Equal(t, route.NetworkBytes, route.RelayNetworkBytes)
		// ingress 1s + validation 1s precedes egress 2s + extraction 2s.
		require.InDelta(t, 1, route.Stages[0].AfterSeconds, 1e-9)
		require.InDelta(t, 2, route.Stages[1].AfterSeconds, 1e-9)
	}
}

func TestCloudPeerAndLocalAvoidRelay(t *testing.T) {
	m := testModel()
	peer, err := m.Route("cloud-a", "cloud-b", 100)
	require.NoError(t, err)
	require.Equal(t, "direct-runtime", peer.Strategy)
	require.Len(t, peer.Stages, 1)
	require.EqualValues(t, 50, peer.NetworkBytes)
	require.Zero(t, peer.RelayNetworkBytes)
	require.InDelta(t, 2, peer.Stages[0].BeforeSeconds, 1e-9)
	local, err := m.Route("cloud-a", "cloud-a", 100)
	require.NoError(t, err)
	require.Equal(t, "runtime-local", local.Strategy)
	require.Empty(t, local.Stages)
	require.Zero(t, local.NetworkBytes)
}

func TestSharedStorageNeedsProofOnBothNodes(t *testing.T) {
	m := testModel()
	a := m.Resources["hpc-a"]
	a.SharedStorageID = "nfs-evidence"
	a.SharedStorageProven = true
	m.Resources["hpc-a"] = a
	b := m.Resources["hpc-b"]
	b.SharedStorageID = "nfs-evidence"
	m.Resources["hpc-b"] = b
	_, err := m.Route("hpc-a", "hpc-b", 100)
	require.ErrorContains(t, err, "no evidenced gateway profile")
	b.SharedStorageProven = true
	m.Resources["hpc-b"] = b
	shared, err := m.Route("hpc-a", "hpc-b", 100)
	require.NoError(t, err)
	require.Equal(t, "shared-storage", shared.Strategy)
	require.Zero(t, shared.NetworkBytes)
	require.InDelta(t, 3, shared.Stages[0].BeforeSeconds+shared.Stages[0].AfterSeconds, 1e-9)
}

func TestMergeChecksAllInputButCopiesUnionOnce(t *testing.T) {
	m := testModel()
	seconds, err := m.MergeSeconds("cloud-b", 250, 150)
	require.NoError(t, err)
	require.InDelta(t, 10, seconds, 1e-9) // check 2.5, copy 3, verify 4.5.
	zero, err := m.MergeSeconds("hpc-a", 0, 0)
	require.NoError(t, err)
	require.Zero(t, zero)
	_, err = m.MergeSeconds("cloud-a", 100, 200)
	require.Error(t, err)
}

func TestMergeCommandOverheadChargedOnceAndNotForRoots(t *testing.T) {
	m := testModel()
	m.MergeProfiles[0].CommandOverheadSeconds = 7
	seconds, err := m.MergeSeconds("cloud-b", 250, 150)
	require.NoError(t, err)
	require.InDelta(t, 17, seconds, 1e-9)
	zero, err := m.MergeSeconds("cloud-b", 0, 0)
	require.NoError(t, err)
	require.Zero(t, zero)
	m.MergeProfiles[0].CommandOverheadSeconds = -1
	require.Error(t, m.Validate())
}

func TestModelRejectsUnknownRatesAndNonfiniteInputs(t *testing.T) {
	m := testModel()
	_, err := m.Route("unknown", "cloud-a", 100)
	require.Error(t, err)
	_, err = m.Route("hpc-a", "hpc-b", 100)
	require.Error(t, err)
	_, err = m.Route("cloud-a", "cloud-b", -1)
	require.Error(t, err)
	_, err = m.Route("cloud-a", "cloud-b", math.MaxInt64)
	require.ErrorContains(t, err, "overflow")
	m.Profiles[0].UploadArchiveBytesPerSecond = math.Inf(1)
	require.Error(t, m.Validate())
	m = testModel()
	m.Profiles[1].IngressIncludesSourcePreparation = false
	require.Error(t, m.Validate())
	m = testModel()
	m.Profiles = append(m.Profiles, m.Profiles[0])
	require.Error(t, m.Validate())
	legacy, err := Decode(nil)
	require.NoError(t, err)
	require.Nil(t, legacy)
	decoded, err := Decode(map[string]any{"workspaceMaterializationModel": testModel()})
	require.NoError(t, err)
	require.NotNil(t, decoded)
}
