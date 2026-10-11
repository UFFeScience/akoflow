package montagetools

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func fitsHeader(reader io.Reader) Object {
	values := Object{}
	card := make([]byte, 80)
	for i := 0; i < 16384; i++ {
		_, e := io.ReadFull(reader, card)
		check(e == nil, "Truncated FITS header")
		key := strings.TrimSpace(string(card[:8]))
		if key == "END" {
			return values
		}
		if key == "BITPIX" || key == "NAXIS" || key == "NAXIS1" || key == "NAXIS2" {
			check(string(card[8:10]) == "= ", "Invalid FITS numeric card")
			v, e := strconv.Atoi(strings.TrimSpace(strings.SplitN(string(card[10:]), "/", 2)[0]))
			check(e == nil, "Invalid FITS numeric card")
			values[key] = v
		}
	}
	panic("FITS header exceeds bounded input limit")
}
func inputFeature(h Object) Object {
	check(num(get(h, "NAXIS")) == 2, "Expected two-dimensional FITS input")
	x, y := num(get(h, "NAXIS1")), num(get(h, "NAXIS2"))
	check(x > 0 && y > 0, "Invalid image dimensions")
	return Object{"unit": "input-megapixels", "computeWorkUnits": x * y / 1000000, "naxis1": x, "naxis2": y, "bitpix": h["BITPIX"]}
}

// shellWords only parses command arguments; it never executes them.
func shellWords(text string) []string {
	result := []string{}
	var word strings.Builder
	quote := rune(0)
	escaped, started := false, false
	for _, r := range text {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			if started {
				result = append(result, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	check(quote == 0 && !escaped, "Unmatched shell quoting")
	if started {
		result = append(result, word.String())
	}
	return result
}
func workFeatures(request, inputs Object) Object {
	paths := index(get(inputs, "inputs"), "path")
	dependencies := Object{}
	workflow := obj(get(request, "workflow"))
	for _, d := range rows(get(workflow, "dependencies")) {
		id := str(d["activityId"])
		dependencies[id] = append(arr(dependencies[id]), d)
	}
	result := Object{}
	for _, a := range rows(get(workflow, "activities")) {
		id := str(get(a, "id"))
		f := family(id)
		check(strings.Contains("|mproject|mdifffit|mconcatfit|mbgmodel|mbackground|mimgtbl|madd|mviewer|", "|"+f+"|"), "Unknown Montage family")
		arguments := arr(get(obj(get(a, "command")), "arguments"))
		check(len(arguments) > 0, "Missing activity command")
		words := shellWords(str(arguments[len(arguments)-1]))
		unit, work := "fixed-dataset-operation", 1.0
		if f == "mproject" {
			source := []string{}
			for _, w := range words {
				if paths[w] != nil {
					source = append(source, w)
				}
			}
			check(len(source) == 1, "Ambiguous mProject input work")
			feature := obj(paths[source[0]])
			f += "_" + str(get(feature, "band"))
			unit = str(get(feature, "unit"))
			work = num(get(feature, "computeWorkUnits"))
		} else if f == "mconcatfit" {
			check(dependencies[id] != nil, "Missing predecessor work")
			unit, work = "predecessor-fit-record", float64(len(arr(dependencies[id])))
		} else if f == "mviewer" {
			channels := 0
			for _, w := range words {
				if w == "-red" || w == "-green" || w == "-blue" || w == "-gray" {
					channels++
				}
			}
			check(channels == 1 || channels == 3, "Missing viewer channel work")
			unit, work = "image-channel", float64(channels)
		}
		check(work > 0 && !math.IsNaN(work) && !math.IsInf(work, 0), "Invalid independent compute work")
		result[id] = Object{"activityName": get(a, "name"), "activityFamily": f, "computeWorkUnits": work, "unit": unit, "sourceImageSHA256": get(inputs, "sourceImageSHA256")}
	}
	return result
}

var imageDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var inputPathPattern = regexp.MustCompile(`^poss2ukstu_(blue|red|ir)_\d{3}_\d{3}\.fits$`)

func dockerRead(args ...string) []byte {
	cmd := exec.Command("docker", args...)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	out, e := cmd.Output()
	check(e == nil, "Scientific container read failed (diagnostic output withheld)")
	return out
}
func extractWork(container, image string, p Object) Object {
	listing := string(dockerRead("exec", container, "apptainer", "sif", "list", image))
	offset := ""
	for _, line := range strings.Split(listing, "\n") {
		if strings.Contains(line, "FS (Squashfs/") {
			parts := strings.Split(line, "|")
			check(len(parts) > 3, "Invalid SIF inventory")
			value, e := strconv.Atoi(strings.TrimSpace(strings.SplitN(strings.TrimSpace(parts[3]), "-", 2)[0]))
			check(e == nil, "Invalid SIF partition offset")
			offset = strconv.Itoa(value)
			break
		}
	}
	check(offset != "", "Missing Squashfs partition")
	hashFields := strings.Fields(string(dockerRead("exec", container, "sha256sum", image)))
	check(len(hashFields) > 0 && imageDigestPattern.MatchString(hashFields[0]), "Invalid source image identity")
	models := obj(get(p, "activityModels"))
	keys := sortedKeys(models)
	check(len(keys) > 0, "Missing packaging model")
	packaging := rows(get(obj(models[keys[0]]), "inputs"))
	inputs := []Object{}
	for _, f := range packaging {
		if inputPathPattern.MatchString(str(f["path"])) {
			inputs = append(inputs, f)
		}
	}
	check(len(inputs) == 12, "Expected twelve immutable input images")
	sortRows(inputs, "path")
	features := []any{}
	for _, f := range inputs {
		path := str(f["path"])
		compressed := dockerRead("exec", container, "unsquashfs", "-o", offset, "-cat", image, "akoflow-wfa-shared/"+path)
		check("sha256:"+digest(compressed) == f["contentId"], "Input content identity differs from audited dataset")
		gz, e := gzip.NewReader(bytes.NewReader(compressed))
		check(e == nil, "Invalid scientific gzip input")
		header := fitsHeader(gz)
		check(gz.Close() == nil, "Invalid gzip input")
		feature := inputFeature(header)
		feature["path"] = path
		feature["contentId"] = f["contentId"]
		feature["compressedBytes"] = f["sizeBytes"]
		feature["band"] = strings.Split(path, "_")[1]
		features = append(features, feature)
	}
	return Object{"schemaVersion": 1, "sourceImageSHA256": hashFields[0], "basis": "immutable-input-FITS-header", "inputs": features, "limitations": list("Pixel count is input work, not measured CPU service demand; band and hardware may need separate calibrated coefficients.")}
}
func supplementRoots(c, p, request, response Object, d string) Object {
	check(get(c, "computeWorkBasis") == "immutable-dataset-independent-work-v1", "Independent compute units required")
	check(obj(get(response, "run"))["status"] == "failed", "Expected failed training experiment")
	work := Object{}
	for _, v := range obj(get(p, "activityComputeWork")) {
		f := obj(v)
		work[str(get(f, "activityName"))] = f
	}
	roots := []Object{}
	for _, a := range rows(get(obj(get(request, "workflow")), "activities")) {
		if strings.HasPrefix(str(get(a, "name")), "mprojectid") {
			roots = append(roots, a)
		}
	}
	check(len(roots) == 12, "Need twelve Montage roots")
	assignments := index(get(obj(get(request, "plan")), "assignments"), "activityId")
	samples := []Object{}
	for _, a := range roots {
		id := str(a["id"])
		tasks, handles := []Object{}, []Object{}
		for _, t := range rows(get(response, "activityAttempts")) {
			if t["activityId"] == id {
				tasks = append(tasks, t)
			}
		}
		for _, h := range rows(get(response, "handles")) {
			if h["activityId"] == id {
				handles = append(handles, h)
			}
		}
		check(len(tasks) == 1 && num(tasks[0]["attempt"]) == 1 && tasks[0]["status"] == "completed", "Root is not a successful single attempt")
		check(len(handles) == 1 && handles[0]["status"] == "completed" && handles[0]["exitCode"] != nil && num(handles[0]["exitCode"]) == 0 && handles[0]["id"] == str(obj(response["run"])["id"])+":"+id, "Missing exact successful root handle")
		check(len(arr(get(obj(get(handles[0], "artifacts")), "finalSnapshot"))) >= 2, "Missing physical root output snapshot")
		resource := str(get(obj(get(assignments, id)), "resourceId"))
		check(strings.Contains(resource, "bora"), "Training roots must preserve Bora resource family")
		seconds := num(get(tasks[0], "runtimeSeconds"))
		check(seconds > 0 && !math.IsInf(seconds, 0) && !math.IsNaN(seconds), "Invalid observed root runtime")
		f := obj(get(work, str(a["name"])))
		samples = append(samples, Object{"activityFamily": get(f, "activityFamily"), "resourceId": resource, "runtimeSeconds": seconds, "secondsPerWorkUnit": seconds / num(get(f, "computeWorkUnits"))})
	}
	result := clone(c)
	source := str(obj(response["run"])["id"]) + ":sha256:" + d
	result["computeSupplementSource"] = source
	observations := arr(get(result, "computeObservations"))
	for _, r := range rows(observations) {
		if strings.HasPrefix(str(r["activityFamily"]), "mproject_") && r["resourceClass"] == "hpc" {
			r["allowedResourceIds"] = list("plafrim-execution-ssh-node-diablo03")
		}
	}
	for _, g := range groups(samples, "activityFamily", "resourceId") {
		observations = append(observations, Object{"activityFamily": g.key[0], "resourceClass": "hpc", "allowedResourceIds": list(g.key[1]), "source": source, "taskRuntimeFallbackSamples": len(g.rows), "runtimeSeconds": summary(vals(g.rows, "runtimeSeconds")), "wallSecondsPerReferenceSecond": summary(vals(g.rows, "secondsPerWorkUnit")), "interpretation": "Completed single-attempt root observations only; source run failed later and is not end-to-end validation."})
	}
	result["computeObservations"] = observations
	return result
}
func numericString(v any) string { return fmt.Sprintf("%v", v) }
