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

	"github.com/UFFeScience/akoflow/internal/domain"
)

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

// Exercise real rsync and the remote shell script, with transport redirected
// into temporary directories. This is not a live Lustre integration test.
func TestMixedWorkspaceRoutesConflictAndRecovery(t *testing.T) {
	realRsync, err := exec.LookPath("rsync")
	if err != nil {
		t.Skip("rsync unavailable")
	}
	bin := t.TempDir()
	ssh := "#!/bin/sh\nwhile [ \"$#\" -gt 1 ]; do shift; done\ncase \"$1\" in *'stat -f'*) printf 'nfs 0 nfs4 server:/fixture /home\\nnfs 0 nfs4 server:/fixture /home\\n'; exit 0;; esac\nexec sh -c \"$1\"\n"
	rsync := fmt.Sprintf("#!/bin/bash\nif [[ -n ${AKOFLOW_TEST_FAIL_ONCE:-} && ! -e $AKOFLOW_TEST_FAIL_ONCE ]]; then touch \"$AKOFLOW_TEST_FAIL_ONCE\"; echo 'injected network interruption' >&2; exit 12; fi\nargs=(); skip=0\nfor arg in \"$@\"; do if ((skip)); then skip=0; continue; fi; case \"$arg\" in -e) skip=1;; --secluded-args) ;; hpc.test:*) args+=(\"${arg#hpc.test:}\");; *) args+=(\"$arg\");; esac; done\nexec %s \"${args[@]}\"\n", shell(realRsync))
	for name, body := range map[string]string{"ssh": ssh, "rsync": rsync} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", t.TempDir())
	root := t.TempDir()
	hpc, fog, target := filepath.Join(root, "hpc"), filepath.Join(root, "fog"), filepath.Join(root, "target")
	for _, path := range []string{hpc, fog, target} {
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
	remote := func(path string) string { return (&url.URL{Scheme: "ssh", Host: "hpc.test", Path: path}).String() }
	plans := []domain.DataTransferPlan{
		{ID: "hpc", Source: domain.TransferLocation{URI: remote(hpc)}, Destination: domain.TransferLocation{URI: remote(target)}},
		{ID: "fog", Source: domain.TransferLocation{URI: workspaceURL(fog)}, Destination: domain.TransferLocation{URI: remote(target)}},
	}
	syncer := WorkspaceRsync{Resolver: directWorkspaceEndpointResolver{}}
	runs, err := syncer.Sync(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	if runs[0].NetworkBytes != 0 || !strings.Contains(runs[0].Route.Reason, "verified shared") || runs[1].NetworkBytes <= 0 {
		t.Fatalf("mixed routes: %+v", runs)
	}
	if runs[0].Route.SharedFilesystem != "nfs" || !strings.Contains(runs[0].Route.SharedFilesystemID, "server:/fixture") || runs[1].Route.SharedFilesystem != "" || runs[0].Route.FilesTransferred != runs[0].FilesTransferred {
		t.Fatalf("missing or incorrect route evidence: %+v", runs)
	}
	for _, name := range []string{"hpc.fits", "fog.fits"} {
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
