---
id: examples
title: Examples
sidebar_label: Examples
---

All examples below are ready to copy, paste, and submit. They live in the repository at [akoflow-workflow-engine/pkg/client/resource/examples/](https://github.com/UFFeScience/akoflow/tree/main/akoflow-workflow-engine/pkg/client/resource/examples/).

**How to submit any example:**

```bash
# encode the YAML and send it to the running engine
curl -s -X POST http://localhost:8080/api/v1/workflow \
     -H "Content-Type: application/json" \
     -d "{\"workflow\": \"$(base64 -i my-workflow.yaml)\"}"
```

---

## 01 — Hello World

The minimum viable workflow. Three tasks: two run in parallel, one collects their outputs.

```
hello-a ──┐
           ├──▶ combine
hello-b ──┘
```

**Requirements:** local Kubernetes (Docker Desktop, kind, or minikube)

```yaml
name: wf-hello-world
spec:
  image: "alpine:latest"
  namespace: "akoflow"
  storagePolicy:
    type: distributed
    storageClassName: "hostpath"
    storageSize: "32Mi"
  mountPath: "/data"

  activities:
    - name: "hello-a"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        echo "Hello from task A"
        echo "task-a" > /data/a.txt

    - name: "hello-b"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        echo "Hello from task B"
        echo "task-b" > /data/b.txt

    - name: "combine"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["hello-a", "hello-b"]
      run: |
        echo "=== Result ==="
        cat /data/a.txt /data/b.txt
        echo "done"
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/01-hello-world.yaml)

---

## 02 — Sequential Pipeline

A linear chain where each task depends on the previous one. Data is passed between tasks via the shared `mountPath` volume.

```
ingest → clean → transform → report
```

**Requirements:** local Kubernetes

```yaml
name: wf-sequential-pipeline
spec:
  image: "alpine:latest"
  namespace: "akoflow"
  storagePolicy:
    type: standalone
    storageClassName: "hostpath"
    storageSize: "64Mi"
  mountPath: "/data"

  activities:
    - name: "ingest"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        seq 1 100 > /data/raw.csv
        echo "Ingested $(wc -l < /data/raw.csv) records"

    - name: "clean"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["ingest"]
      run: |
        awk 'NR%2==0' /data/raw.csv > /data/clean.csv
        echo "Kept $(wc -l < /data/clean.csv) records after cleaning"

    - name: "transform"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["clean"]
      run: |
        awk '{print $1 * 2}' /data/clean.csv > /data/transformed.csv
        echo "Transformation complete"

    - name: "report"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["transform"]
      run: |
        echo "Raw:       $(wc -l < /data/raw.csv)"
        echo "Clean:     $(wc -l < /data/clean.csv)"
        echo "Transformed: $(wc -l < /data/transformed.csv)"
        head -5 /data/transformed.csv
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/02-sequential-pipeline.yaml)

---

## 03 — Fan-Out / Fan-In (Scatter-Gather)

One task splits the work into chunks; multiple tasks process them in parallel; one task aggregates the results. This is the standard map-reduce pattern.

```
         ┌──▶ process-chunk-1 ──┐
split ───┼──▶ process-chunk-2 ──┤
         ├──▶ process-chunk-3 ──┼──▶ aggregate
         └──▶ process-chunk-4 ──┘
```

**Requirements:** local Kubernetes

```yaml
name: wf-fan-out-fan-in
spec:
  image: "alpine:latest"
  namespace: "akoflow"
  storagePolicy:
    type: standalone
    storageClassName: "hostpath"
    storageSize: "64Mi"
  mountPath: "/data"

  activities:
    - name: "split"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        seq 1 400 > /data/full.csv
        split -l 100 /data/full.csv /data/chunk-

    - name: "process-chunk-1"
      memoryLimit: 128Mi
      cpuLimit: 0.2
      dependsOn: ["split"]
      run: |
        awk '{sum += $1} END {print sum}' /data/chunk-aa > /data/result-1.txt

    - name: "process-chunk-2"
      memoryLimit: 128Mi
      cpuLimit: 0.2
      dependsOn: ["split"]
      run: |
        awk '{sum += $1} END {print sum}' /data/chunk-ab > /data/result-2.txt

    - name: "process-chunk-3"
      memoryLimit: 128Mi
      cpuLimit: 0.2
      dependsOn: ["split"]
      run: |
        awk '{sum += $1} END {print sum}' /data/chunk-ac > /data/result-3.txt

    - name: "process-chunk-4"
      memoryLimit: 128Mi
      cpuLimit: 0.2
      dependsOn: ["split"]
      run: |
        awk '{sum += $1} END {print sum}' /data/chunk-ad > /data/result-4.txt

    - name: "aggregate"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn:
        - "process-chunk-1"
        - "process-chunk-2"
        - "process-chunk-3"
        - "process-chunk-4"
      run: |
        cat /data/result-{1,2,3,4}.txt \
          | awk '{total += $1} END {print "Grand total:", total}'
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/03-fan-out-fan-in.yaml)

---

## 04 — Diamond DAG

The canonical DAG correctness test: two parallel branches both depend on the same source and converge at the same sink. Useful for verifying that AkôFlow respects all edges when deciding readiness.

```
        ┌──▶ branch-left  ──┐
start ──┤                   ├──▶ merge
        └──▶ branch-right ──┘
```

**Requirements:** local Kubernetes

```yaml
name: wf-diamond
spec:
  image: "alpine:latest"
  namespace: "akoflow"
  storagePolicy:
    type: standalone
    storageClassName: "hostpath"
    storageSize: "32Mi"
  mountPath: "/data"

  activities:
    - name: "start"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        echo "shared-input" > /data/input.txt

    - name: "branch-left"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["start"]
      run: |
        echo "left:$(cat /data/input.txt)" > /data/left.txt

    - name: "branch-right"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["start"]
      run: |
        echo "right:$(cat /data/input.txt)" > /data/right.txt

    - name: "merge"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["branch-left", "branch-right"]
      run: |
        cat /data/left.txt /data/right.txt
        echo "Merged successfully"
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/04-diamond-dag.yaml)

---

## 05 — Python ETL Pipeline

A realistic ETL workflow using Python. Demonstrates: shared `standalone` volume for data passing between tasks, parallel validation and profiling branches, and a final reporting step.

```
extract ──┬──▶ validate ──┐
          └──▶ profile  ──┴──▶ transform ──▶ load ──▶ report
```

**Requirements:** local Kubernetes + internet (pulls `python:3.11-slim`)

```yaml
name: wf-python-etl
spec:
  image: "python:3.11-slim"
  namespace: "akoflow"
  storagePolicy:
    type: standalone
    storageClassName: "hostpath"
    storageSize: "128Mi"
  mountPath: "/data"

  activities:
    - name: "extract"
      memoryLimit: 256Mi
      cpuLimit: 0.5
      run: |
        python3 - <<'EOF'
        import json, random
        random.seed(42)
        rows = [{"id": i, "value": round(random.gauss(50,15),2),
                 "region": random.choice(["us","eu","apac"]),
                 "valid": random.random() > 0.1} for i in range(200)]
        with open("/data/raw.json","w") as f: json.dump(rows, f)
        print(f"Extracted {len(rows)} records")
        EOF

    - name: "validate"
      memoryLimit: 256Mi
      cpuLimit: 0.3
      dependsOn: ["extract"]
      run: |
        python3 - <<'EOF'
        import json
        with open("/data/raw.json") as f: rows = json.load(f)
        valid = [r for r in rows if r["valid"] and r["value"] >= 0]
        with open("/data/valid.json","w") as f: json.dump(valid, f)
        print(f"Valid: {len(valid)}/{len(rows)}")
        EOF

    - name: "profile"
      memoryLimit: 256Mi
      cpuLimit: 0.3
      dependsOn: ["extract"]
      run: |
        python3 - <<'EOF'
        import json, statistics
        with open("/data/raw.json") as f: rows = json.load(f)
        values = [r["value"] for r in rows]
        profile = {"count":len(values),"mean":round(statistics.mean(values),2),
                   "stdev":round(statistics.stdev(values),2)}
        with open("/data/profile.json","w") as f: json.dump(profile, f)
        print("Profile:", profile)
        EOF

    - name: "transform"
      memoryLimit: 256Mi
      cpuLimit: 0.5
      dependsOn: ["validate", "profile"]
      run: |
        python3 - <<'EOF'
        import json
        with open("/data/valid.json") as f: rows = json.load(f)
        with open("/data/profile.json") as f: p = json.load(f)
        for r in rows:
            r["normalized"] = round((r["value"] - p["mean"]) / p["stdev"], 4)
        with open("/data/transformed.json","w") as f: json.dump(rows, f)
        print(f"Transformed {len(rows)} records")
        EOF

    - name: "load"
      memoryLimit: 256Mi
      cpuLimit: 0.3
      dependsOn: ["transform"]
      run: |
        python3 - <<'EOF'
        import json, csv
        with open("/data/transformed.json") as f: rows = json.load(f)
        with open("/data/output.csv","w",newline="") as f:
            w = csv.DictWriter(f, fieldnames=["id","region","value","normalized"])
            w.writeheader()
            [w.writerow({k:r[k] for k in ["id","region","value","normalized"]}) for r in rows]
        print(f"Wrote {len(rows)} rows to output.csv")
        EOF

    - name: "report"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["load"]
      run: |
        python3 - <<'EOF'
        import csv, json
        with open("/data/output.csv") as f: rows = list(csv.DictReader(f))
        with open("/data/profile.json") as f: p = json.load(f)
        print(f"Records: {len(rows)}  Mean: {p['mean']}  Stdev: {p['stdev']}")
        [print(f"  {r}") for r in rows[:3]]
        EOF
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/05-python-etl.yaml)

---

## 06 — Multi-Runtime (Cross-Cloud)

Different tasks run on different infrastructure backends in the same workflow. Pre-processing runs locally, two cloud regions process their respective datasets in parallel, and a final step aggregates.

```
prepare (local) ──┬──▶ process-us (k8s-aws) ──┐
                  └──▶ process-eu (k8s-gcp) ──┴──▶ aggregate (local)
```

**Requirements:** two configured Kubernetes runtimes — replace `k8s-aws` / `k8s-gcp` with your actual runtime names.

```yaml
name: wf-multi-runtime
spec:
  image: "python:3.11-slim"
  namespace: "akoflow"
  runtime: "local"
  storagePolicy:
    type: standalone
    storageClassName: "hostpath"
    storageSize: "128Mi"
  mountPath: "/data"

  activities:
    - name: "prepare"
      runtime: "local"
      memoryLimit: 256Mi
      cpuLimit: 0.5
      run: |
        python3 - <<'EOF'
        import json, random
        random.seed(1)
        us = [{"id":i,"region":"us","value":random.randint(1,100)} for i in range(50)]
        eu = [{"id":i,"region":"eu","value":random.randint(1,100)} for i in range(50)]
        with open("/data/us-input.json","w") as f: json.dump(us, f)
        with open("/data/eu-input.json","w") as f: json.dump(eu, f)
        print(f"Prepared {len(us)} US and {len(eu)} EU records")
        EOF

    - name: "process-us"
      runtime: "k8s-aws"
      memoryLimit: 512Mi
      cpuLimit: 1.0
      dependsOn: ["prepare"]
      run: |
        python3 - <<'EOF'
        import json, statistics
        with open("/data/us-input.json") as f: rows = json.load(f)
        v = [r["value"] for r in rows]
        result = {"region":"us","count":len(rows),"mean":round(statistics.mean(v),2),"total":sum(v)}
        with open("/data/us-result.json","w") as f: json.dump(result, f)
        print("US:", result)
        EOF

    - name: "process-eu"
      runtime: "k8s-gcp"
      memoryLimit: 512Mi
      cpuLimit: 1.0
      dependsOn: ["prepare"]
      run: |
        python3 - <<'EOF'
        import json, statistics
        with open("/data/eu-input.json") as f: rows = json.load(f)
        v = [r["value"] for r in rows]
        result = {"region":"eu","count":len(rows),"mean":round(statistics.mean(v),2),"total":sum(v)}
        with open("/data/eu-result.json","w") as f: json.dump(result, f)
        print("EU:", result)
        EOF

    - name: "aggregate"
      runtime: "local"
      memoryLimit: 256Mi
      cpuLimit: 0.3
      dependsOn: ["process-us", "process-eu"]
      run: |
        python3 - <<'EOF'
        import json
        us = json.load(open("/data/us-result.json"))
        eu = json.load(open("/data/eu-result.json"))
        print(f"US:  count={us['count']}  mean={us['mean']}  total={us['total']}")
        print(f"EU:  count={eu['count']}  mean={eu['mean']}  total={eu['total']}")
        print(f"ALL: count={us['count']+eu['count']}  total={us['total']+eu['total']}")
        EOF
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/06-multi-runtime.yaml)

---

## 07 — Singularity on HPC (SLURM)

Tasks submitted as Singularity containers to a SLURM cluster via SSH. The Engine connects remotely — it does not need to run on the HPC cluster.

```
preprocess → compute → postprocess
```

**Requirements:** VPN access to HPC, Singularity `.sif` on remote filesystem, `HPC_*` env vars configured.

```yaml
name: wf-singularity-hpc
spec:
  runtime: "hpc-sdumont"          # replace with your runtime name
  image: "/scratch/myuser/sifs/myapp.sif"
  storagePolicy:
    type: default
  mountPath: "/scratch/myuser/akoflow"

  activities:
    - name: "preprocess"
      runtime: "hpc-sdumont"
      memoryLimit: 4Gi
      cpuLimit: 4
      run: |
        python3 preprocess.py --input /scratch/data/raw --output /scratch/data/clean

    - name: "compute"
      runtime: "hpc-sdumont"
      memoryLimit: 16Gi
      cpuLimit: 16
      dependsOn: ["preprocess"]
      run: |
        python3 compute.py --input /scratch/data/clean --output /scratch/data/result

    - name: "postprocess"
      runtime: "hpc-sdumont"
      memoryLimit: 4Gi
      cpuLimit: 4
      dependsOn: ["compute"]
      run: |
        python3 postprocess.py --input /scratch/data/result --output /scratch/data/final
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/07-singularity-hpc.yaml)

---

## 08 — Stress Test

A synthetic workflow with five parallel tasks of varying memory and duration, designed to stress-test the scheduler and monitor. Use it to compare AkôScore behavior under different `α` values.

```
init ──┬──▶ compute-1 (CPU-bound)  ──┐
       ├──▶ compute-2 (5s sleep)   ──┤
       ├──▶ compute-3 (8s sleep)   ──┼──▶ finalize
       ├──▶ compute-4 (3s sleep)   ──┤
       └──▶ compute-5 (6s sleep)   ──┘
```

**Requirements:** local Kubernetes

```yaml
name: wf-stress-test
spec:
  image: "alpine:latest"
  namespace: "akoflow"
  storagePolicy:
    type: distributed
    storageClassName: "hostpath"
    storageSize: "32Mi"
  mountPath: "/data"

  activities:
    - name: "init"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        echo "$(date +%s)" > /data/start.txt

    - name: "compute-1"
      memoryLimit: 256Mi
      cpuLimit: 0.5
      dependsOn: ["init"]
      run: |
        i=0; while [ $i -lt 1000000 ]; do i=$((i+1)); done
        echo "compute-1:done" > /data/c1.txt

    - name: "compute-2"
      memoryLimit: 512Mi
      cpuLimit: 0.5
      dependsOn: ["init"]
      run: |
        sleep 5
        echo "compute-2:done" > /data/c2.txt

    - name: "compute-3"
      memoryLimit: 256Mi
      cpuLimit: 1.0
      dependsOn: ["init"]
      run: |
        sleep 8
        echo "compute-3:done" > /data/c3.txt

    - name: "compute-4"
      memoryLimit: 128Mi
      cpuLimit: 0.3
      dependsOn: ["init"]
      run: |
        sleep 3
        echo "compute-4:done" > /data/c4.txt

    - name: "compute-5"
      memoryLimit: 512Mi
      cpuLimit: 0.5
      dependsOn: ["init"]
      run: |
        sleep 6
        echo "compute-5:done" > /data/c5.txt

    - name: "finalize"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn:
        - "compute-1"
        - "compute-2"
        - "compute-3"
        - "compute-4"
        - "compute-5"
      run: |
        start=$(cat /data/start.txt)
        echo "All tasks done in $(($(date +%s) - start))s"
        cat /data/c1.txt /data/c2.txt /data/c3.txt /data/c4.txt /data/c5.txt
```

→ [View file](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/examples/08-stress-test.yaml)

---

## Montage — Astronomical Image Mosaic

The Montage workflow is a real-world scientific workflow used to create image mosaics from astronomical survey data. It is the standard benchmark for scientific workflow systems.

The full workflow processes three color bands (blue, red, infrared) with a shared final composition step, resulting in a colored mosaic image.

→ [View full workflow](https://github.com/UFFeScience/akoflow/blob/main/akoflow-workflow-engine/pkg/client/resource/v0-06-nfsserver/wf-1-montage-distributed.yaml)

```yaml
name: wf-montage-distributed
spec:
  image: "ovvesley/akoflow-wf-montage:050d"
  namespace: "akoflow"
  storagePolicy:
    type: distributed
    storageClassName: "hostpath"
    storageSize: "4Gi"
  mountPath: "/data"

  activities:
    # Blue band
    - name: mprojectid0000001
      run: mProject -X poss2ukstu_blue_001_001.fits pposs2ukstu_blue_001_001.fits region-oversized.hdr
      memoryLimit: 256Mi
      cpuLimit: 500m

    # ... (57 total activities: project → diff → fit → bgmodel → background → add → view)

    # Final color composition (depends on all three band mosaics)
    - name: mviewerid0000058
      run: mViewer -red 3-mosaic.fits -green 2-mosaic.fits -blue 1-mosaic.fits -png mosaic-color.png
      memoryLimit: 256Mi
      cpuLimit: 500m
      dependsOn:
        - maddid0000018   # blue mosaic
        - maddid0000037   # red mosaic
        - maddid0000056   # IR mosaic
```
