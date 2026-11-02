package sshfilesystem

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const quotaFixture = `__AKO_DF__
Filesystem 1024-blocks Used Available Capacity Mounted on
server:/home 100000000 5000000 95000000 5% /home
__AKO_UID__
12345
__AKO_QUOTA__
quota: Cannot resolve unrelated mount: Permission denied
Disk quotas for user researcher:
Filesystem blocks quota limit grace files quota limit grace
server:/home 20971340 20971520 20971520 0 2894 4294967295 4294967295 0
__AKO_END__
`

func TestEffectiveWorkspaceBudgetUsesPersonalQuotaNotFilesystemFree(t *testing.T) {
	bytes, _, _, err := parseWorkspaceBudget(quotaFixture)
	require.NoError(t, err)
	require.Equal(t, int64(180*1024), bytes)
	for _, replacement := range []string{"20971520* 20971520 20971520", "20971521* 20971520 20971520"} {
		value := strings.Replace(quotaFixture, "20971340 20971520 20971520", replacement, 1)
		bytes, _, _, err = parseWorkspaceBudget(value)
		require.NoError(t, err)
		require.Zero(t, bytes)
	}
}

func TestBudgetRejectsUnprovenQuotaMappingOrTruncatedOutput(t *testing.T) {
	for _, value := range []string{
		strings.Replace(quotaFixture, "server:/home 20971340", "server:/different 20971340", 1),
		strings.Replace(quotaFixture, "__AKO_END__", "", 1),
		strings.Replace(quotaFixture, "20971340", "invalid", 1),
	} {
		_, _, _, err := parseWorkspaceBudget(value)
		require.Error(t, err)
	}
}

func TestWorkspaceBudgetHasBoundedReadOnlySSHAndStableAccountIdentity(t *testing.T) {
	executor := &commandFake{output: []byte(quotaFixture)}
	driver := New(connectionStoreFake{connection: testConnection()}, executor)
	value, err := driver.WorkspaceBudget(context.Background(), testStorage(), "")
	require.NoError(t, err)
	require.Equal(t, int64(180*1024), value.AvailableBytes)
	require.NotContains(t, value.StorageID, "server")
	storage := testStorage()
	storage.ID = "another-logical-node"
	second, err := driver.WorkspaceBudget(context.Background(), storage, "")
	require.NoError(t, err)
	require.Equal(t, value.StorageID, second.StorageID)
	_, err = driver.WorkspaceBudget(context.Background(), storage, "../../etc")
	require.Error(t, err)
}

func TestBeeGFSRequiresExactUserAndGroupLimits(t *testing.T) {
	prefix := strings.Split(quotaFixture, "__AKO_QUOTA__")[0]
	fixture := prefix + "__AKO_QUOTA__\n__AKO_BEE_QUOTA__ 12345\nname,id,size,hard,files,hard\nuser,12345,1000,100000,0,unlimited\n__AKO_BEE_QUOTA__ 50\ngroup,50,500,unlimited,0,unlimited\n__AKO_END__\n"
	bytes, _, _, e := parseWorkspaceBudget(fixture)
	require.NoError(t, e)
	require.Equal(t, int64(96*1024), bytes)
	limited := strings.Replace(fixture, "group,50,500,unlimited", "group,50,500,5000", 1)
	bytes, _, _, e = parseWorkspaceBudget(limited)
	require.NoError(t, e)
	require.Equal(t, int64(4*1024), bytes)
	for _, bad := range []string{strings.Replace(fixture, "group,50,500,unlimited", "group,51,500,unlimited", 1), strings.Replace(fixture, "1000,100000", "1000,unknown", 1), strings.Replace(fixture, "__AKO_END__", "", 1), strings.Replace(fixture, "__AKO_BEE_QUOTA__ 50", "user,12345,0,200000,0,unlimited\n__AKO_BEE_QUOTA__ 50", 1)} {
		_, _, _, e = parseWorkspaceBudget(bad)
		require.Error(t, e)
	}
}

func TestBeeGFSExhaustedQuotaDoesNotBecomeUnlimited(t *testing.T) {
	prefix := strings.Split(quotaFixture, "__AKO_QUOTA__")[0]
	input := prefix + "__AKO_QUOTA__\n__AKO_BEE_QUOTA__ 12345\nuser,12345,100,100,0,unlimited\n__AKO_BEE_QUOTA__ 50\ngroup,50,0,unlimited,0,unlimited\n__AKO_END__\n"
	bytes, _, _, e := parseWorkspaceBudget(input)
	require.NoError(t, e)
	require.Zero(t, bytes)
}
