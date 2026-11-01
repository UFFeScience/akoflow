//go:build ignore

// Compare scientific PNG products across the selected physical deployments.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func load(p string) (image.Image, string) {
	b, e := os.ReadFile(p)
	must(e)
	im, e := png.Decode(bytes.NewReader(b))
	must(e)
	return im, fmt.Sprintf("sha256:%x", sha256.Sum256(b))
}
func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		panic("ROOT [DEPLOYMENTS_JSON]")
	}
	root := os.Args[1]
	directories := map[string]string{}
	mediumDirectory := ""
	baselineDirectory := filepath.Join(root, "heft-hpc")
	groups := []string{"prism-hpc-real", "prism-heft-medium", "moheft-medium"}
	for _, group := range groups {
		directories[group] = filepath.Join(root, group)
	}
	if len(os.Args) == 3 {
		var sources map[string]struct{ Directory, RunID string }
		b, e := os.ReadFile(os.Args[2])
		must(e)
		must(json.Unmarshal(b, &sources))
		if len(sources) != 6 || sources["heft-hpc"].Directory == "" {
			panic("six independent deployment sources required")
		}
		baselineDirectory = sources["heft-hpc"].Directory
		mediumDirectory = sources["prism-time-medium"].Directory
		groups = nil
		seen := map[string]bool{}
		for key, source := range sources {
			if source.RunID == "" || seen[source.RunID] {
				panic("distinct real runs required for product crosscheck")
			}
			seen[source.RunID] = true
			directories[key] = source.Directory
			if key != "heft-hpc" {
				groups = append(groups, key)
			}
		}
		sort.Strings(groups)
		for _, algorithm := range []string{"heft", "moheft"} {
			group := "within-medium/" + algorithm
			groups = append(groups, group)
			directories[group] = sources[algorithm+"-medium"].Directory
		}
	}
	baseline := map[string]string{}
	files, e := filepath.Glob(filepath.Join(baselineDirectory, "final-products/*/*.png"))
	must(e)
	if len(files) != 4 {
		panic("four baseline PNGs required")
	}
	for _, p := range files {
		baseline[filepath.Base(p)] = p
	}
	mediumBaseline := map[string]string{}
	if mediumDirectory != "" {
		files, e := filepath.Glob(filepath.Join(mediumDirectory, "final-products/*/*.png"))
		must(e)
		if len(files) != 4 {
			panic("four medium reference PNGs required")
		}
		for _, p := range files {
			mediumBaseline[filepath.Base(p)] = p
		}
	}
	rows := []map[string]any{}
	for _, group := range groups {
		files, e := filepath.Glob(filepath.Join(directories[group], "final-products/*/*.png"))
		must(e)
		if len(files) != 4 {
			panic("four PNGs required for " + group)
		}
		for _, p := range files {
			references := baseline
			if strings.HasPrefix(group, "within-medium/") {
				references = mediumBaseline
			}
			reference, ok := references[filepath.Base(p)]
			if !ok {
				panic("unexpected final product")
			}
			a, ah := load(reference)
			b, bh := load(p)
			if a.Bounds() != b.Bounds() {
				panic("PNG dimensions differ")
			}
			var pixels, different int64
			peak, sumSquared := 0.0, 0.0
			bounds := a.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					ar, ag, ab, aa := a.At(x, y).RGBA()
					br, bg, bb, ba := b.At(x, y).RGBA()
					av, bv := []uint32{ar, ag, ab, aa}, []uint32{br, bg, bb, ba}
					changed := false
					for k := range av {
						d := math.Abs(float64(av[k])-float64(bv[k])) / 257
						peak = math.Max(peak, d)
						sumSquared += d * d
						if av[k] != bv[k] {
							changed = true
						}
					}
					pixels++
					if changed {
						different++
					}
				}
			}
			rows = append(rows, map[string]any{"group": group, "product": filepath.Base(p), "baselinePath": reference, "path": p, "baselineSHA256": ah, "sha256": bh, "byteIdentical": ah == bh, "pixelIdentical": different == 0, "differentPixels": different, "pixelCount": pixels, "differentPixelFraction": float64(different) / float64(pixels), "maximumChannelDifference8bit": peak, "channelRMSE8bit": math.Sqrt(sumSquared / float64(4*pixels))})
		}
	}
	b, e := json.MarshalIndent(rows, "", "  ")
	must(e)
	must(os.WriteFile(filepath.Join(root, "final-products-crosscheck.json"), b, 0600))
	fmt.Printf("Compared %d PNG pairs; includes independent within-medium comparisons when supplied\n", len(rows))
}
