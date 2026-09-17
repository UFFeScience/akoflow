package transfer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Run explicitly with AKOFLOW_LIVE_MONTAGE_SOURCE=ssh://... pointing at a
// completed mProject workspace on a real GCP VM. The test only creates and
// removes its own temporary directory on that VM.
func TestWorkspaceRsyncLiveMontageGCPToLocal(t *testing.T) {
	uri := os.Getenv("AKOFLOW_LIVE_MONTAGE_SOURCE")
	if uri == "" {
		t.Skip("set AKOFLOW_LIVE_MONTAGE_SOURCE for the live GCP integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	remoteProducer, remoteLocalSeed := stageLiveMontageInputs(t, ctx, uri)
	root := t.TempDir()
	localProducer, successor := filepath.Join(root, "local-producer"), filepath.Join(root, "successor")
	if err := os.Mkdir(localProducer, 0700); err != nil {
		t.Fatal(err)
	}
	remoteURI := func(path string) string { u, _ := url.Parse(uri); u.Path = path; return u.String() }
	if _, err := syncDirectory(ctx, domain.TransferEndpoint{URI: remoteURI(remoteLocalSeed)}, domain.TransferEndpoint{URI: workspaceURL(localProducer)}, false); err != nil {
		t.Fatalf("prepare local predecessor: %v", err)
	}
	image := os.Getenv("AKOFLOW_LIVE_MONTAGE_IMAGE")
	if image == "" {
		image = "ovvesley/akoflow-wf-montage:050d"
	}
	projectArgs := []string{"run", "--rm", "--network", "none", "-v", localProducer + ":/akoflow-wfa-shared", "-w", "/akoflow-wfa-shared", image,
		"mProject", "-X", "poss2ukstu_blue_002_001.fits", "pposs2ukstu_blue_002_001.fits", "region-oversized.hdr"}
	if output, err := exec.CommandContext(ctx, "docker", projectArgs...).CombinedOutput(); err != nil {
		t.Fatalf("local predecessor mProject failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	plans := []domain.DataTransferPlan{
		{
			ID: "gcp-to-local", ExecutionRunID: "montage-live",
			ProducerActivityID: "mproject-gcp", ConsumerActivityID: "mdifffit-local",
			Source:      domain.TransferLocation{URI: remoteURI(remoteProducer), ResourceID: "gcp-vm"},
			Destination: domain.TransferLocation{URI: workspaceURL(successor), ResourceID: "local"},
		},
		{
			ID: "local-to-local", ExecutionRunID: "montage-live",
			ProducerActivityID: "mproject-local", ConsumerActivityID: "mdifffit-local",
			Source:      domain.TransferLocation{URI: workspaceURL(localProducer), ResourceID: "local"},
			Destination: domain.TransferLocation{URI: workspaceURL(successor), ResourceID: "local"},
		},
	}
	runs, err := (WorkspaceRsync{Resolver: workspaceEndpointResolver{}}).Sync(ctx, plans)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("workspace ready before command: GCP files=%d bytes=%d network=%d, local files=%d", runs[0].FilesTransferred, runs[0].TransferredBytes, runs[0].NetworkBytes, runs[1].FilesTransferred)
	for _, name := range []string{"pposs2ukstu_blue_002_001.fits", "pposs2ukstu_blue_002_002.fits", "region-oversized.hdr"} {
		if _, err := os.Stat(filepath.Join(successor, name)); err != nil {
			t.Fatalf("input %q absent before activity command: %v", name, err)
		}
	}
	args := []string{"run", "--rm", "--network", "none", "-v", successor + ":/akoflow-wfa-shared", "-w", "/akoflow-wfa-shared", image,
		"mDiffFit", "-d", "-s", "1-fit.000003.000004.txt", "pposs2ukstu_blue_002_001.fits", "pposs2ukstu_blue_002_002.fits", "1-diff.000003.000004.fits", "region-oversized.hdr"}
	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("real local Montage successor failed after sync: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if _, err := os.Stat(filepath.Join(successor, "1-fit.000003.000004.txt")); err != nil {
		t.Fatalf("successor output missing: %v", err)
	}
	t.Logf("Montage successor completed: GCP transfer %d file/%d bytes, local predecessor %d files, output present", runs[0].FilesTransferred, runs[0].TransferredBytes, runs[1].FilesTransferred)
}

func stageLiveMontageInputs(t *testing.T, ctx context.Context, uri string) (string, string) {
	t.Helper()
	endpoint := domain.TransferEndpoint{URI: uri}
	host, sourcePath, err := sshTarget(endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	runSSH := func(command string) (string, error) {
		output, runErr := exec.CommandContext(ctx, "ssh", append(sshArgs(endpoint), host, command)...).CombinedOutput()
		return strings.TrimSpace(string(output)), runErr
	}
	remoteRoot, err := runSSH("mktemp -d /tmp/akoflow-rsync-montage-XXXXXXXX")
	if err != nil {
		t.Fatalf("create remote test workspace: %v: %s", err, remoteRoot)
	}
	if !strings.HasPrefix(remoteRoot, "/tmp/akoflow-rsync-montage-") || strings.ContainsAny(remoteRoot, " \t\r\n") {
		t.Fatalf("invalid temporary workspace path")
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = exec.CommandContext(cleanupCtx, "ssh", append(sshArgs(endpoint), host, "rm -rf -- "+shell(remoteRoot))...).Run()
	})
	remoteProducer := remoteRoot + "/gcp-producer"
	remoteLocalSeed := remoteRoot + "/local-seed"
	command := fmt.Sprintf("mkdir -p -- %s %s && cp -- %s %s && cp -- %s %s && cp -- %s %s && cp -- %s %s",
		shell(remoteProducer), shell(remoteLocalSeed),
		shell(filepath.Join(sourcePath, "pposs2ukstu_blue_002_002.fits")), shell(remoteProducer),
		shell(filepath.Join(sourcePath, "pposs2ukstu_blue_002_002_area.fits")), shell(remoteProducer),
		shell(filepath.Join(sourcePath, "poss2ukstu_blue_002_001.fits")), shell(remoteLocalSeed),
		shell(filepath.Join(sourcePath, "region-oversized.hdr")), shell(remoteLocalSeed))
	if output, err := runSSH(command); err != nil {
		t.Fatalf("stage real Montage inputs: %v: %s", err, output)
	}
	return remoteProducer, remoteLocalSeed
}
