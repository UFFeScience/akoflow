//go:build ignore

// Preserve only the four final PNG products, without archiving GBs of inputs.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbexecution "github.com/UFFeScience/akoflow/internal/infrastructure/database/execution"
	"github.com/UFFeScience/akoflow/internal/infrastructure/instancearchive"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type obj = map[string]any

func must(e error) {
	if e != nil {
		panic(e)
	}
}

var client = &http.Client{Timeout: 90 * time.Second}
var apiBase = "http://127.0.0.1:5173/akoflow-api"

func durableProduct(path string, data []byte) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(parent, ".export-*.partial")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(parent)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func call(method, path string, input any, output any) error {
	var b []byte
	var e error
	if input != nil {
		b, e = json.Marshal(input)
		if e != nil {
			return e
		}
	}
	r, e := http.NewRequest(method, apiBase+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	r.Header.Set("Content-Type", "application/json")
	if t := os.Getenv("AKOFLOW_API_TOKEN"); t != "" {
		r.Header.Set("Authorization", "Bearer "+t)
	}
	s, e := client.Do(r)
	if e != nil {
		return e
	}
	defer s.Body.Close()
	if s.StatusCode >= 300 {
		return fmt.Errorf("%s %s HTTP %d", method, path, s.StatusCode)
	}
	if output != nil {
		return json.NewDecoder(s.Body).Decode(output)
	}
	return nil
}
func main() {
	if len(os.Args) != 4 {
		panic("RUN_ID ENVIRONMENT CAMPAIGN_DIRECTORY")
	}
	run, env, dir := os.Args[1], os.Args[2], os.Args[3]
	if value := os.Getenv("FIG8_API"); value != "" {
		apiBase = value
	}
	var store *dbexecution.Repository
	if os.Getenv("FIG8_EXPORT_LEASES") == "1" {
		if instancearchive.IsReadOnlySelection() {
			panic("cannot lease archival database")
		}
		db, e := database.Open(instancearchive.ResolveDatabasePath())
		must(e)
		defer db.Close()
		store = dbexecution.New(db)
	}
	registered := map[string]domain.WorkspaceLease{}
	if store != nil {
		leases, err := store.ListWorkspaceLeases(context.Background(), run)
		must(err)
		for _, lease := range leases {
			if lease.ConsumerActivityID == "verified-final-product-export" {
				registered[lease.ProducerActivityID] = lease
			}
		}
	}
	prefix := "montage-independent-work-v1-mviewerid00000"
	wanted := map[string]string{prefix + "19": "1-mosaic.png", prefix + "38": "2-mosaic.png", prefix + "57": "3-mosaic.png", prefix + "58": "mosaic-color.png"}
	proofs := map[string]obj{}
	restarted := map[string]bool{}
	for {
		var trace struct {
			Run        struct{ ID, Status string }
			Workspaces []struct {
				ID, ActivityID, ResourceID, ConnectionID, ExecutionPath, URI string
				Manifest                                                     struct {
					Outputs []struct {
						Path, Digest string
						SizeBytes    int64
					}
				}
			}
		}
		if err := call("GET", "/execution-runs/"+run+"/monitor/", nil, &trace); err != nil {
			fmt.Println("Observation pending; retaining the same execution", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if trace.Run.ID != run {
			panic("wrong run")
		}
		for _, w := range trace.Workspaces {
			name, needed := wanted[w.ActivityID]
			if !needed || proofs[w.ActivityID] != nil {
				continue
			}
			if store != nil && registered[w.ActivityID].ID == "" {
				lease := domain.WorkspaceLease{ID: run + "-product-export-" + w.ActivityID, WorkspaceID: w.ID, RunID: run, ProducerActivityID: w.ActivityID, ConsumerActivityID: "verified-final-product-export"}
				must(store.SaveWorkspaceLease(context.Background(), lease))
				registered[w.ActivityID] = lease
			}
			var digest string
			var size int64
			for _, f := range w.Manifest.Outputs {
				if f.Path == name {
					digest, size = f.Digest, f.SizeBytes
				}
			}
			if digest == "" {
				continue
			}
			if size <= 0 || size > 128<<20 {
				panic("unexpected PNG size")
			}
			target := filepath.Join(dir, "final-products", w.ActivityID, name)
			if data, err := os.ReadFile(target); err == nil {
				got := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
				if int64(len(data)) == size && got == digest {
					if _, err := png.DecodeConfig(bytes.NewReader(data)); err == nil {
						if store != nil {
							lease := registered[w.ActivityID]
							now := time.Now().UTC()
							lease.Released, lease.ReleasedAt = true, &now
							lease.ReleaseReason = "Final PNG exported and verified: " + got + " " + target
							must(store.SaveWorkspaceLease(context.Background(), lease))
						}
						proofs[w.ActivityID] = obj{"resourceId": w.ResourceID, "path": target, "sha256": got, "sizeBytes": size}
						continue
					}
				}
			}
			storage, path := "", ""
			if strings.Contains(w.ResourceID, "diablo03") {
				var storages []struct{ ID, Endpoint string }
				if e := call("GET", "/environments/"+env+"/storages/", nil, &storages); e != nil {
					continue
				}
				for _, s := range storages {
					if s.Endpoint == "/home/wferreir" {
						storage = s.ID
						break
					}
				}
				root := "/home/wferreir/akoflow-workspaces/" + run + "/"
				if !strings.HasPrefix(w.ExecutionPath, root) {
					panic("unbounded HPC output")
				}
				path = strings.TrimPrefix(w.ExecutionPath, "/home/wferreir/") + "/" + name
			} else {
				marker := "cloudInstanceId="
				index := strings.Index(w.URI, marker)
				if index < 0 {
					continue
				}
				id := strings.Split(w.URI[index+len(marker):], "&")[0]
				if !strings.HasPrefix(id, "cloud-instance-") {
					panic("invalid cloud generation")
				}
				var instance struct{ ID, Status string }
				if e := call("GET", "/cloud-instances/"+id+"/", nil, &instance); e != nil {
					continue
				}
				if instance.Status == "stopped" && trace.Run.Status == "completed" && !restarted[id] {
					fmt.Println("Starting stopped generation solely for post-run product export", id)
					if e := call("POST", "/cloud-instances/"+id+"/start/", obj{}, nil); e != nil {
						continue
					}
					restarted[id] = true
					continue
				}
				if instance.Status != "ready" {
					continue
				}
				storage = "cloud-workspace:" + id
				path = "runs/" + run + "/" + w.ActivityID + "/" + name
			}
			if storage == "" {
				continue
			}
			download := "products-" + run + "-" + w.ActivityID
			var result struct{ ID, Status string }
			if e := call("POST", "/storages/"+storage+"/downloads/", obj{"id": download, "path": path}, &result); e != nil {
				fmt.Println("Product pending", w.ActivityID, e)
				continue
			}
			req, e := http.NewRequest("GET", apiBase+"/storage-downloads/"+download+"/content/", nil)
			must(e)
			if t := os.Getenv("AKOFLOW_API_TOKEN"); t != "" {
				req.Header.Set("Authorization", "Bearer "+t)
			}
			response, e := client.Do(req)
			if e != nil {
				continue
			}
			if response.StatusCode != 200 {
				response.Body.Close()
				continue
			}
			data, e := io.ReadAll(io.LimitReader(response.Body, 128<<20))
			response.Body.Close()
			if e != nil {
				continue
			}
			got := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
			if int64(len(data)) != size || got != digest {
				panic("PNG differs from published manifest")
			}
			if _, e = png.DecodeConfig(bytes.NewReader(data)); e != nil {
				panic(e)
			}
			must(durableProduct(target, data))
			if store != nil {
				lease := registered[w.ActivityID]
				now := time.Now().UTC()
				lease.Released = true
				lease.ReleasedAt = &now
				lease.ReleaseReason = "Final PNG exported and verified: " + got + " " + target
				must(store.SaveWorkspaceLease(context.Background(), lease))
			}
			proofs[w.ActivityID] = obj{"resourceId": w.ResourceID, "storageId": storage, "path": target, "sha256": got, "sizeBytes": size}
			fmt.Println("Verified PNG", w.ActivityID, size)
		}
		complete := trace.Run.Status == "completed" && len(proofs) == 4
		b, e := json.MarshalIndent(obj{"runId": run, "status": trace.Run.Status, "complete": complete, "products": proofs, "postRunRestartedInstances": restarted}, "", "  ")
		must(e)
		must(os.WriteFile(filepath.Join(dir, "final-products-audit.json"), b, 0600))
		if complete {
			return
		}
		if trace.Run.Status == "failed" || trace.Run.Status == "cancelled" {
			panic("run ended without complete products")
		}
		time.Sleep(3 * time.Second)
	}
}
