package transfer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestWorkspacePhaseRecordsFailureWithoutChangingError(t *testing.T) {
	var run domain.DataTransferRun
	want := errors.New("transport interrupted")
	if got := measureWorkspacePhase(&run, "archive-upload", func() error { return want }); got != want {
		t.Fatalf("error changed: %v", got)
	}
	phase := run.Route.Phases[0]
	if !phase.Failed || phase.Name != "archive-upload" || phase.FinishedAt < phase.StartedAt || phase.DurationSeconds < 0 {
		t.Fatalf("invalid phase: %+v", phase)
	}
}

func TestRemoteWorkspaceVerificationReadsUniqueDestinationOnce(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatal("python3 is required for the remote-script fixture")
	}
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint("corrupt=", corrupt), func(t *testing.T) {
			root := t.TempDir()
			bin, dest := filepath.Join(root, "bin"), filepath.Join(root, "dest")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			var sources []string
			for i := 0; i < 3; i++ {
				source := filepath.Join(root, fmt.Sprint("source", i))
				if err := os.Mkdir(source, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "shared.fits"), []byte("same content"), 0600); err != nil {
					t.Fatal(err)
				}
				sources = append(sources, source)
			}
			// Only copying is a fixture. Python hashes actual contents and rejects
			// corruption injected after all predecessor checks.
			copy := "#!/bin/bash\nargs=(\"$@\"); src=${args[${#args[@]}-2]}; dst=${args[${#args[@]}-1]}; mkdir -p \"$dst\"; if [[ ! -e $dst/shared.fits ]]; then cp \"$src/shared.fits\" \"$dst/shared.fits\"; fi\n"
			if corrupt {
				copy += "printf 'corrupt' >\"$dst/shared.fits\"\n"
			}
			if err := os.WriteFile(filepath.Join(bin, "rsync"), []byte(copy), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-c", remoteWorkspaceScript(sources, dest))
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
			output, runErr := cmd.CombinedOutput()
			if corrupt {
				if runErr == nil || !strings.Contains(string(output), "workspace verification failed") {
					t.Fatalf("corruption accepted: %v %s", runErr, output)
				}
			} else if runErr != nil {
				t.Fatalf("merge failed: %v %s", runErr, output)
			}
			if !corrupt && !strings.Contains(string(output), "__AKOFLOW_MERGE_VERIFIED_FILES__=1") {
				t.Fatalf("destination not verified once per unique path: %s", output)
			}
		})
	}
}

func TestSharedWorkspaceProofRequiresMatchingFilesystemAndMount(t *testing.T) {
	for _, test := range []struct {
		name, output string
		want         bool
	}{
		{"shared Lustre", "lustre a12 lustre server:/lustre /home\nlustre a12 lustre server:/lustre /home\n", true},
		{"shared NFS zero ID", "nfs 0 nfs4 server:/home /home\nnfs 0 nfs4 server:/home /home\n", true},
		{"shared BeeGFS", "fhgfs 0 beegfs beegfs_nodev /beegfs\nfhgfs 0 beegfs beegfs_nodev /beegfs\n", true},
		{"different mount source", "nfs 0 nfs server:/a /home\nnfs 0 nfs server:/b /home\n", false},
		{"different mount path", "nfs 0 nfs server:/a /home\nnfs 0 nfs server:/a /scratch\n", false},
		{"different filesystem identity", "lustre a12 lustre server:/a /home\nlustre b34 lustre server:/a /home\n", false},
		{"local filesystem", "ext4 a12 ext4 /dev/sda /home\next4 a12 ext4 /dev/sda /home\n", false},
		{"filesystem type mismatch", "nfs 0 ext4 /dev/sda /home\nnfs 0 ext4 /dev/sda /home\n", false},
		{"not Lustre", "ext4 a12\next4 a12\n", false},
		{"missing destination", "lustre a12\n", false},
		{"missing identity", "lustre\nlustre\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, got := parseSharedWorkspaceProof(test.output)
			if got != test.want {
				t.Fatalf("proof accepted=%v want=%v", got, test.want)
			}
		})
	}
}

func TestSameCloudWorkspaceRequiresExactGenerationAndTrust(t *testing.T) {
	u := "ssh://worker.test/workspace?strictHostKeyChecking=true&hostKeyAlias=generation-a&knownHostsFile=/trusted/hosts"
	a := domain.TransferEndpoint{URI: u, ConnectionID: "cloud-instance-a", CloudInstanceID: "instance-a"}
	b := a
	if !sameVerifiedCloudWorkspaceEndpoint(a, b) {
		t.Fatal("verified same generation rejected")
	}
	b.CloudInstanceID = "instance-b"
	if sameVerifiedCloudWorkspaceEndpoint(a, b) {
		t.Fatal("different instance accepted")
	}
	b = a
	b.CloudInstanceID = ""
	if sameVerifiedCloudWorkspaceEndpoint(a, b) {
		t.Fatal("missing instance accepted")
	}
	b = a
	b.URI = "ssh://worker.test/workspace"
	if sameVerifiedCloudWorkspaceEndpoint(a, b) {
		t.Fatal("untrusted endpoint accepted")
	}
	b = a
	b.URI = strings.Replace(u, "generation-a", "generation-b", 1)
	if sameVerifiedCloudWorkspaceEndpoint(a, b) {
		t.Fatal("different trust generation accepted")
	}
}

func TestSameCloudWorkspaceLocalFilesystemProof(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte("#!/bin/sh\nprintf 'ext2/ext3 a12 ext4 /dev/sda /akoflow\\next2/ext3 a12 ext4 /dev/sda /akoflow\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", t.TempDir())
	u := "ssh://worker.test/workspace?strictHostKeyChecking=true&hostKeyAlias=generation-a&knownHostsFile=/trusted/hosts"
	a := domain.TransferEndpoint{URI: u, ConnectionID: "cloud-a", CloudInstanceID: "instance-a"}
	if proof, ok := sameCloudWorkspaceProof(context.Background(), a, a); !ok || !strings.HasPrefix(proof, "ext2/ext3 ") {
		t.Fatalf("local filesystem proof rejected: %q %v", proof, ok)
	}
}

// Exercise real rsync and the remote shell script, with transport redirected
// into temporary directories. This is not a live Lustre integration test.
func TestMixedWorkspaceRoutesConflictAndRecovery(t *testing.T) {
	for _, sameCloud := range []bool{false, true} {
		t.Run(fmt.Sprintf("sameCloud=%v", sameCloud), func(t *testing.T) { testMixedWorkspaceRoutesConflictAndRecovery(t, sameCloud) })
	}
}

func testMixedWorkspaceRoutesConflictAndRecovery(t *testing.T, sameCloud bool) {
	realRsync, err := exec.LookPath("rsync")
	if err != nil {
		t.Skip("rsync unavailable")
	}
	bin := t.TempDir()
	// Strip the GNU-only archive option in this local BSD-tar transport fixture.
	ssh := "#!/bin/sh\nwhile [ \"$#\" -gt 1 ]; do shift; done\ncase \"$1\" in *'stat -f'*) printf 'nfs 0 nfs4 server:/fixture /home\\nnfs 0 nfs4 server:/fixture /home\\n'; exit 0;; esac\ncommand=$(printf '%s' \"$1\" | sed 's/--hard-dereference //g'); exec sh -c \"$command\"\n"
	if sameCloud {
		ssh = strings.ReplaceAll(ssh, "nfs 0 nfs4 server:/fixture /home", "ext2/ext3 a12 ext4 /dev/sda /akoflow")
	}
	rsync := fmt.Sprintf("#!/bin/bash\nif [[ -n ${AKOFLOW_TEST_FAIL_ONCE:-} && ! -e $AKOFLOW_TEST_FAIL_ONCE ]]; then touch \"$AKOFLOW_TEST_FAIL_ONCE\"; echo 'injected network interruption' >&2; exit 12; fi\nargs=(); skip=0\nfor arg in \"$@\"; do if ((skip)); then skip=0; continue; fi; case \"$arg\" in -e) skip=1;; --secluded-args) ;; hpc.test:*) args+=(\"${arg#hpc.test:}\");; *) args+=(\"$arg\");; esac; done\nexec %s \"${args[@]}\"\n", shell(realRsync))
	for name, body := range map[string]string{"ssh": ssh, "rsync": rsync} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", t.TempDir())
	root := t.TempDir()
	hpc, fog, fog2, target := filepath.Join(root, "hpc"), filepath.Join(root, "fog"), filepath.Join(root, "fog2"), filepath.Join(root, "target")
	for _, path := range []string{hpc, fog, fog2, target} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(hpc, "hpc.fits"), "HPC")
	write(filepath.Join(fog, "fog.fits"), "FOG")
	write(filepath.Join(fog2, "fog2.fits"), "FOG2")
	remote := func(path string) string {
		u := &url.URL{Scheme: "ssh", Host: "hpc.test", Path: path}
		if sameCloud {
			u.RawQuery = "strictHostKeyChecking=true&hostKeyAlias=verified-generation&knownHostsFile=/trusted/hosts"
		}
		return u.String()
	}
	plans := []domain.DataTransferPlan{
		{ID: "hpc", Source: domain.TransferLocation{URI: remote(hpc)}, Destination: domain.TransferLocation{URI: remote(target)}},
		{ID: "fog", Source: domain.TransferLocation{URI: workspaceURL(fog)}, Destination: domain.TransferLocation{URI: remote(target)}},
		{ID: "fog2", Source: domain.TransferLocation{URI: (&url.URL{Scheme: "ssh", Host: "cloud.test", Path: fog2}).String()}, Destination: domain.TransferLocation{URI: remote(target)}},
	}
	if sameCloud {
		for i := range plans {
			plans[i].Destination.CloudInstanceID = "verified-instance"
		}
		plans[0].Source.CloudInstanceID = "verified-instance"
	}
	syncer := WorkspaceRsync{Resolver: directWorkspaceEndpointResolver{}}
	runs, err := syncer.Sync(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	expectedReason, expectedFS, expectedMount := "verified shared", "nfs", "server:/fixture"
	if sameCloud {
		expectedReason, expectedFS, expectedMount = "verified same concrete cloud", "ext2/ext3", "/dev/sda"
		if runs[0].Strategy != domain.TransferRuntimeLocal {
			t.Fatalf("same cloud did not use runtime-local: %+v", runs[0])
		}
	}
	if runs[0].NetworkBytes != 0 || !strings.Contains(runs[0].Route.Reason, expectedReason) || runs[1].NetworkBytes <= 0 {
		t.Fatalf("mixed routes: %+v", runs)
	}
	for _, run := range runs[1:] {
		if run.Route.ArchiveBytes <= 0 || len(run.Route.Phases) != 6 {
			t.Fatalf("missing phase/archive evidence: %+v", run.Route)
		}
		if run.Route.Packaging != "tar-gzip" || run.Route.PreparationSeconds <= 0 || run.NetworkBytes <= 0 {
			t.Fatalf("missing archive preparation evidence: %+v", run)
		}
	}
	if len(runs[0].Route.Phases) != 2 || runs[0].Route.ArchiveBytes != 0 {
		t.Fatalf("shared origin incorrectly attributed archive work: %+v", runs[0].Route)
	}
	if runs[0].Route.SharedFilesystem != expectedFS || !strings.Contains(runs[0].Route.SharedFilesystemID, expectedMount) || runs[1].Route.SharedFilesystem != "" || runs[0].Route.FilesTransferred != runs[0].FilesTransferred {
		t.Fatalf("missing or incorrect route evidence: %+v", runs)
	}
	for _, name := range []string{"hpc.fits", "fog.fits", "fog2.fits"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatal(err)
		}
	}
	// Consumer mutation must not modify producer data (no hardlinks).
	write(filepath.Join(target, "hpc.fits"), "consumer mutation")
	content, _ := os.ReadFile(filepath.Join(hpc, "hpc.fits"))
	if string(content) != "HPC" {
		t.Fatal("producer mutated")
	}
	if _, err := syncer.Sync(context.Background(), plans); err == nil {
		t.Fatal("conflict accepted")
	}
	write(filepath.Join(target, "hpc.fits"), "HPC")
	runs, err = syncer.Sync(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	if runs[0].FilesTransferred != 0 || runs[1].FilesTransferred != 0 {
		t.Fatalf("valid destination recopied: %+v", runs)
	}
	// An interrupted network route must not release the merge barrier. A retry
	// preserves producer data and reuses files already validated at destination.
	write(filepath.Join(fog, "after-interruption"), "new")
	t.Setenv("AKOFLOW_TEST_FAIL_ONCE", filepath.Join(root, "failure-injected"))
	failed, err := syncer.Sync(context.Background(), plans)
	if err == nil || failed[1].Status != domain.TransferFailed || failed[1].Error == "" {
		t.Fatalf("interruption not recorded: runs=%+v err=%v", failed, err)
	}
	if _, err := os.Stat(filepath.Join(target, "after-interruption")); !os.IsNotExist(err) {
		t.Fatal("published after failed network preparation")
	}
	runs, err = syncer.Sync(context.Background(), plans)
	if err != nil || runs[0].FilesTransferred != 0 || runs[1].FilesTransferred != 1 {
		t.Fatalf("recovery did not reuse valid publication: runs=%+v err=%v", runs, err)
	}
	if content, err := os.ReadFile(filepath.Join(fog, "after-interruption")); err != nil || string(content) != "new" {
		t.Fatal("producer data lost during recovery")
	}
	// A conflict between mixed producers blocks publication of a new file.
	write(filepath.Join(hpc, "collision"), "one")
	write(filepath.Join(fog, "collision"), "two")
	write(filepath.Join(fog, "not-published"), "new")
	if _, err := syncer.Sync(context.Background(), plans); err == nil {
		t.Fatal("mixed conflict accepted")
	}
	if _, err := os.Stat(filepath.Join(target, "not-published")); !os.IsNotExist(err) {
		t.Fatal("published before aggregate validation")
	}
}

func TestWorkspaceRouteAddressRedactsCredentials(t *testing.T) {
	got := workspaceRouteAddress("ssh://user:secret@hpc.test/workspace?identityFile=/secret/key#secret")
	if got != "ssh://hpc.test/workspace" {
		t.Fatalf("unsafe route address: %q", got)
	}
}
