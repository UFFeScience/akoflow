package montagetools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Run returns bounded errors rather than raw API inputs. This CLI never reads
// database.db, submits runs, provisions machines or restarts the daemon.
func Run(args []string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	check(len(args) > 0, "Montage tool command required")
	command := strings.ReplaceAll(args[0], "-", "_")
	args = args[1:]
	load := func(i int) Object { m, _ := read(args[i]); return m }
	arity := func(n int, usage string) { check(len(args) == n, usage) }
	switch command {
	case "audit_montage_results":
		check(len(args) == 4 || len(args) == 5, "audit_montage_results REQUEST REAL SIM PRODUCT [OUTPUT]")
		result := audit(load(0), load(1), load(2), os.Getenv("MONTAGE_REQUIRE_FRESH") == "1")
		p, e := os.ReadFile(args[3])
		check(e == nil, "read product failed")
		result["product"] = Object{"filename": filepath.Base(args[3]), "bytes": len(p), "sha256": digest(p)}
		if len(args) == 5 {
			write(args[4], result)
		} else {
			fmt.Print(string(encode(result)))
		}
	case "build_montage_execution_calibration":
		check(len(args) == 2 || len(args) == 3, "build_montage_execution_calibration AUDIT OUTPUT [PROJECTION]")
		data, p := read(args[0])
		var projection Object
		if len(args) == 3 {
			projection = load(2)
		}
		write(args[1], calibration(data, digest(p), projection))
	case "build_montage_workspace_projection":
		arity(4, "build_montage_workspace_projection AUDIT API REQUEST OUTPUT")
		a, p := read(args[0])
		write(args[3], projection(a, load(1), load(2), digest(p)))
	case "freeze_montage_materialization":
		arity(4, "freeze_montage_materialization CALIBRATION RESOURCES RELAY OUTPUT")
		c, p := read(args[0])
		write(args[3], freezeMaterialization(c, digest(p), load(1), args[2]))
	case "freeze_montage_compute_profiles":
		arity(4, "freeze_montage_compute_profiles CALIBRATION RESOURCES NAMESPACE OUTPUT")
		c, p := read(args[0])
		write(args[3], freezeCompute(c, digest(p), load(1), args[2]))
	case "build_montage_work_features":
		arity(4, "build_montage_work_features REQUEST INPUTS PROJECTION OUTPUT")
		p := load(2)
		p["activityComputeWork"] = workFeatures(load(0), load(1))
		p["computeWorkBasis"] = "immutable-dataset-independent-work-v1"
		p["limitations"] = append(arr(get(p, "limitations")), "Non-project operation units apply only to this fixed dataset; arbitrary dataset scaling is not calibrated.")
		write(args[3], p)
	case "extract_montage_compute_work":
		arity(4, "extract_montage_compute_work CONTAINER SIF PROJECTION OUTPUT")
		write(args[3], extractWork(args[0], args[1], load(2)))
	case "supplement_montage_root_compute":
		arity(5, "supplement_montage_root_compute CALIBRATION PROJECTION REQUEST RESPONSE OUTPUT")
		response, p := read(args[3])
		write(args[4], supplementRoots(load(0), load(1), load(2), response, digest(p)))
	case "prepare_montage_measured_requests":
		prepareRequests(args)
	case "export_cloud_workspace":
		arity(3, "export_cloud_workspace TARGET RUN OUTPUT.tar.gz")
		exportCloud(args[0], args[1], args[2])
	case "validate_montage_transfer_model":
		check(len(args) > 0, "validation mode required")
		mode := args[0]
		args = args[1:]
		switch mode {
		case "calibrate":
			arity(5, "calibrate AUDIT RAW REQUEST RESOURCES OUTPUT_PREFIX")
			a, p := read(args[0])
			v := measuredVolumes(a, load(1), load(2), digest(p))
			c := calibration(a, digest(p), v)
			m := freezeMaterialization(c, digest(encode(c)), load(3), "montage-relay-controller")
			outputs := Object{"volumes": v, "calibration": c, "model": m}
			for name, value := range outputs {
				write(args[4]+"-"+name+".json", value)
			}
		case "request":
			arity(5, "request ORIGINAL MODEL AUDIT RUN_ID OUTPUT")
			write(args[4], fixedRequest(load(0), load(1), load(2), args[3]))
		case "supplement":
			arity(4, "supplement CALIBRATION DIAGNOSTIC RESOURCES OUTPUT_PREFIX")
			diagnostic, p := read(args[1])
			c := supplementSource(load(0), diagnostic, digest(p))
			resources := load(2)
			for _, r := range resources {
				resource := obj(r)
				if get(resource, "domain") == "hpc" {
					resource["bulkServiceGroupId"] = "plafrim-reference-ssh-bulk"
					resource["bulkServiceSlots"] = 3
					resource["bulkServiceSource"] = "AKO-148 SSHChannelLimit4 reserve1; diagnostic source waits prove two waves of three"
				}
			}
			m := freezeMaterialization(c, digest(encode(c)), resources, "montage-relay-controller")
			write(args[3]+"-calibration.json", c)
			write(args[3]+"-model.json", m)
		case "compare":
			arity(5, "compare AUDIT SIM_RESULT MODEL ROLE OUTPUT")
			result := compare(load(0), load(1), args[3], load(2))
			write(args[4], result)
			delete(result, "dependencies")
			delete(result, "dominantTransfers")
			fmt.Print(string(encode(result)))
		case "shared-capacity":
			arity(4, "shared-capacity REQUEST DIAGNOSTIC HPC_RESOURCE OUTPUT_PREFIX")
			diagnostic, p := read(args[1])
			c := sourceCapacity(diagnostic, args[2], "e2-micro")
			c["runId"] = get(obj(get(diagnostic, "run")), "id")
			c["sha256"] = digest(p)
			write(args[3]+"-request.json", applyCapacity(load(0), args[2], c))
			write(args[3]+"-capacity.json", c)
		case "report":
			arity(2, "report PRIVATE_EXPERIMENT_DIR OUTPUT")
			write(args[1], validationReport(args[0]))
		default:
			panic("Use calibrate, supplement, request, compare, shared-capacity or report; no real runs are submitted")
		}
	default:
		panic("Unknown Montage tool command: " + command)
	}
	return nil
}

func validationReport(base string) Object {
	cases := []any{}
	for _, name := range []string{"ako151-original-comparison", "ako151-historical-comparison", "ako151-clean-v2-comparison", "ako151-shared-source-v2-comparison"} {
		r, _ := read(filepath.Join(base, name+".json"))
		delete(r, "dependencies")
		cases = append(cases, r)
	}
	provenance := Object{}
	for _, name := range []string{"ako151-clean-v2-calibration", "ako151-source-pool-v3-calibration", "ako151-source-pool-v3-model", "ako151-shared-source-v2-capacity"} {
		r, p := read(filepath.Join(base, name+".json"))
		provenance[name] = Object{"sha256": digest(p), "evidence": r}
	}
	previous := Object{}
	for _, name := range []string{"montage-clean-simulation-20261011-v1-request.json", "selected-plan.json", "simulation-results.json"} {
		_, p := read(filepath.Join(base, name))
		previous[name] = digest(p)
	}
	return Object{"schemaVersion": 1, "issue": "AKO-151", "realExecutionStarted": false, "daemonRestarted": false, "validationMode": "native SimGrid offline; exact production export; fixed frozen observed placement and compute", "previousForecastFilesSHA256": previous, "cases": cases, "provenance": provenance, "gate": Object{"accepted": false, "newPlanningReleased": false, "independentRealPrepared": false, "reason": "Shared source-capacity fit meets makespan tolerance but dominant dependency residuals still exceed 100%; do not accept compensating errors."}, "remainingLimitations": list("Receiver command service is not identifiable separately from queue in completed references. No destination pool is added to those mixed samples.", "Nine completed source commands and measured HPC merges from an interrupted diagnostic are auxiliary calibration evidence, not a completed-run benchmark.", "Aggregate source capacity varies with workload/concurrency; constant-rate fit cannot explain every dominant dependency.", "New planning must retain only evidenced route classes. Unobserved placement/storage identity must not be silently inferred.", "Provisioning and orchestration timing retained from the frozen original forecast; not yet independently validated.", "Compute and lifecycle cost bases unchanged; e2-medium interference remains extrapolated and network/public-IP billing remains incomplete.")}
}
