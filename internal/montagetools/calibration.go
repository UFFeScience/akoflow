package montagetools

import "strings"

func validateBaseline(data Object, d string, projection Object) {
	if projection != nil {
		check(eq(projection["baselineRunId"], data["run_id"]) && projection["auditedEvidenceSHA256"] == d, "Workspace projection baseline provenance mismatch")
	}
	audit := obj(get(data, "audit"))
	check(num(get(audit, "expected_count")) == 58 && get(data, "attempt_history") == "single_observed_attempt_per_activity", "Baseline must contain 58 single-attempt completed activities")
	for _, k := range []string{"missing", "unexpected", "not_completed"} {
		check(len(arr(get(audit, k))) == 0, "Unsafe baseline "+k)
	}
	for _, k := range []string{"incomplete_transfer_count", "barrier_violation_count", "unreleased_lease_count", "single_lane_micro_overlap_count", "missing_dependency_route_count", "cross_cloud_hpc_non_relay_count"} {
		check(num(get(audit, k)) == 0, "Unsafe baseline "+k)
	}
	a := rows(get(data, "activities"))
	check(len(a) == 58 && len(index(data["activities"], "activity_id")) == 58, "Duplicate or missing baseline activity")
}
func calibration(data Object, d string, projection Object) Object {
	validateBaseline(data, d, projection)
	activities := rows(data["activities"])
	indexed := index(data["activities"], "activity_id")
	work := obj(projection["activityComputeWork"])
	compute := calibrateComputeObservations(activities, work, projection)
	phases := calibrationPhaseSamples(data, indexed, projection)
	observations := calibratePhaseObservations(phases)
	mergeRows := []Object{}
	for _, p := range rows(get(data, "shared_preparations")) {
		r := clone(p)
		id := str(p["consumer_activity_id"])
		r["family"] = family(id)
		r["class"] = resourceClass(str(obj(get(indexed, id))["resource_id"]))
		mergeRows = append(mergeRows, r)
	}
	merges := []any{}
	for _, g := range groups(mergeRows, "family", "class") {
		merges = append(merges, Object{
			"activityFamily": g.key[0],
			"resourceClass":  g.key[1],
			"mergeSeconds":   summary(vals(g.rows, "merge_seconds")),
			"checkSeconds":   summary(vals(g.rows, "merge_check_seconds")),
			"copySeconds":    summary(vals(g.rows, "merge_copy_seconds")),
			"verifySeconds":  summary(vals(g.rows, "merge_verify_seconds")),
		})
	}
	var mergeRates any
	if projection != nil {
		rates := []any{}
		for _, g := range groups(mergeRows, "class") {
			o := Object{}
			for key, pair := range map[string][2]string{
				"checkSecondsPerInputByte":   {"merge_check_seconds", "inputTotalBytes"},
				"copySecondsPerUniqueByte":   {"merge_copy_seconds", "inputUniqueBytes"},
				"verifySecondsPerUniqueByte": {"merge_verify_seconds", "inputUniqueBytes"},
			} {
				values := []any{}
				for _, r := range g.rows {
					bytes := num(get(obj(get(obj(get(projection, "observedConsumerInputVolumes")), str(r["consumer_activity_id"]))), pair[1]))
					if bytes > 0 {
						values = append(values, num(get(r, pair[0]))/bytes)
					}
				}
				o[key] = summary(values)
			}
			rates = append(rates, Object{
				"targetClass":            g.key[0],
				"source":                 "baseline:" + str(data["run_id"]),
				"observations":           o,
				"commandOverheadSeconds": summary(vals(g.rows, "merge_command_overhead_seconds")),
				"channelWaitSeconds":     summary(vals(g.rows, "merge_ssh_channel_wait_seconds")),
			})
		}
		mergeRates = rates
	}
	basis := "unavailable-do-not-use-logical-output-bytes-as-workspace-volume"
	if projection != nil {
		basis = "producer-final-snapshot-manifest"
	}
	return Object{
		"schemaVersion":                    1,
		"baselineRunId":                    data["run_id"],
		"baselineSHA256":                   d,
		"computeWorkBasis":                 projection["computeWorkBasis"],
		"synchronizationContract":          "whole-workspace-v1",
		"workspaceVolumeBasis":             basis,
		"computeObservations":              compute,
		"materializationPhaseObservations": observations,
		"consumerMergeObservations":        merges,
		"mergeRateObservations":            mergeRates,
		"limitations": list(
			"Single baseline; ranges are observations, not confidence intervals.",
			"Do not replay per-activity durations or total makespan as a holdout prediction.",
			"No destination cache hit or shared-storage route may be assumed without evidence.",
			"Workspace manifests and topology-specific contention are required before applying these observations.",
			"Compute samples are contention-conditioned; do not apply interference slowdown a second time.",
		),
	}
}

func calibrateComputeObservations(activities []Object, work, projection Object) []any {
	computeRows := []Object{}
	for _, a := range activities {
		p := family(str(a["activity_id"]))
		if len(work) > 0 {
			p = str(get(obj(get(work, str(a["activity_id"]))), "activityFamily"))
		}
		r := clone(a)
		r["family"] = p
		r["class"] = resourceClass(str(a["resource_id"]))
		computeRows = append(computeRows, r)
	}
	compute := []any{}
	for _, g := range groups(computeRows, "family", "class") {
		times, normalized := []any{}, []any{}
		fallback := 0
		for _, a := range g.rows {
			runtime := a["validated_handle_runtime_seconds"]
			if runtime == nil {
				runtime = get(a, "observed_runtime_seconds")
				fallback++
			}
			times = append(times, runtime)
			if projection != nil {
				reference := num(get(obj(get(projection, "activityReferenceDurations")), str(a["activity_id"])))
				if len(work) > 0 {
					reference = num(get(obj(get(work, str(a["activity_id"]))), "computeWorkUnits"))
				}
				check(reference > 0, "Invalid reference work")
				normalized = append(normalized, num(runtime)/reference)
			}
		}
		compute = append(compute, Object{
			"activityFamily":                g.key[0],
			"resourceClass":                 g.key[1],
			"runtimeSeconds":                summary(times),
			"taskRuntimeFallbackSamples":    fallback,
			"wallSecondsPerReferenceSecond": summary(normalized),
			"interpretation":                "observed wall runtime; includes physical sharing, not clean CPU service demand",
		})
	}
	return compute
}

func calibrationPhaseSamples(data, indexed, projection Object) []Object {
	phases := []Object{}
	for _, r := range rows(get(data, "routes")) {
		if r["producer_activity_id"] == nil {
			continue
		}
		producer := str(r["producer_activity_id"])
		source := obj(get(indexed, producer))
		target := obj(get(indexed, str(r["consumer_activity_id"])))
		for _, p := range rows(get(r, "phases")) {
			name := str(get(p, "name"))
			if strings.HasPrefix(name, "shared-") {
				continue
			}
			seconds := get(p, "durationSeconds")
			basis := "combined-phase-wall-includes-unseparated-service-queue"
			var phaseWait, sourceWait any
			measured := name == "pack-and-receive" && r["source_pack_execution_seconds"] != nil
			if measured {
				seconds = r["source_pack_execution_seconds"]
				sourceWait = num(r["source_pack_ssh_channel_wait_seconds"])
				basis = "source-command-execution-excludes-channel-queue"
			}
			if name == "archive-upload" && r["rsync_execution_seconds"] != nil {
				seconds = r["rsync_execution_seconds"]
				phaseWait = num(r["ssh_channel_wait_seconds"])
				basis = "phase-command-execution-excludes-channel-queue"
			}
			if p["executionSeconds"] != nil {
				seconds = p["executionSeconds"]
				phaseWait = num(p["sshChannelWaitSeconds"])
				basis = "phase-command-execution-excludes-channel-queue"
			}
			var workspace, preflightWait any
			if projection != nil {
				workspace = get(obj(get(projection, "observedProducerWorkspaceBytes")), producer)
			}
			if r["source_pack_preflight_service_seconds"] != nil {
				preflightWait = num(r["source_pack_preflight_ssh_channel_wait_seconds"])
			}
			phases = append(phases, Object{
				"strategy":                          get(r, "strategy"),
				"sourceClass":                       resourceClass(str(source["resource_id"])),
				"targetClass":                       resourceClass(str(target["resource_id"])),
				"phase":                             name,
				"seconds":                           seconds,
				"secondsBasis":                      basis,
				"phaseChannelWaitSeconds":           phaseWait,
				"sourceChannelWaitSeconds":          sourceWait,
				"sourcePreflightServiceSeconds":     r["source_pack_preflight_service_seconds"],
				"sourcePreflightChannelWaitSeconds": preflightWait,
				"sourcePreflightSeconds":            r["source_pack_preflight_seconds"],
				"logicalBytes":                      r["logical_bytes"],
				"archiveBytes":                      r["archive_bytes"],
				"files":                             r["files"],
				"workspaceBytes":                    workspace,
			})
		}
	}
	return phases
}

func calibratePhaseObservations(phases []Object) []any {
	observations := []any{}
	for _, g := range groups(phases, "strategy", "sourceClass", "targetClass", "phase") {
		bases := Object{}
		ratio, workspaceRate, archiveRate := []any{}, []any{}, []any{}
		for _, p := range g.rows {
			bases[str(p["secondsBasis"])] = true
			if p["archiveBytes"] != nil && num(p["workspaceBytes"]) > 0 {
				ratio = append(ratio, num(p["archiveBytes"])/num(p["workspaceBytes"]))
			}
			if num(p["seconds"]) > 0 {
				if p["workspaceBytes"] != nil {
					workspaceRate = append(workspaceRate, num(p["workspaceBytes"])/num(p["seconds"]))
				}
				if p["archiveBytes"] != nil {
					archiveRate = append(archiveRate, num(p["archiveBytes"])/num(p["seconds"]))
				}
			}
		}
		row := Object{
			"strategy":                         g.key[0],
			"sourceClass":                      g.key[1],
			"targetClass":                      g.key[2],
			"phase":                            g.key[3],
			"seconds":                          summary(vals(g.rows, "seconds")),
			"secondsBases":                     list(sortedKeys(bases)...),
			"pooledArchiveServiceRate":         pooledRate(g.rows, "archiveBytes"),
			"pooledWorkspaceServiceRate":       pooledRate(g.rows, "workspaceBytes"),
			"phaseChannelWaitSeconds":          summary(vals(g.rows, "phaseChannelWaitSeconds")),
			"archiveToWorkspaceRatio":          summary(ratio),
			"effectiveWorkspaceBytesPerSecond": summary(workspaceRate),
			"effectiveArchiveBytesPerSecond":   summary(archiveRate),
			"interpretation":                   "concurrency-conditioned effective phase rates; not isolated link bandwidth",
		}
		for _, k := range []string{"sourceChannelWaitSeconds", "sourcePreflightSeconds", "sourcePreflightServiceSeconds", "sourcePreflightChannelWaitSeconds"} {
			row[k] = nil
			if g.key[3] == "pack-and-receive" {
				row[k] = summary(vals(g.rows, k))
			}
		}
		for _, k := range []string{"logicalBytes", "archiveBytes", "workspaceBytes", "files"} {
			row[k] = summary(vals(g.rows, k))
		}
		observations = append(observations, row)
	}
	return observations
}
