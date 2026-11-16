package workflow

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

const MaxPortableContextBytes = 4 << 20

type ArtifactOrigin struct {
	ExpectedDigest string                 `json:"expectedDigest,omitempty"`
	Image          string                 `json:"image,omitempty"`
	Platform       string                 `json:"platform"`
	Build          *PortableArtifactBuild `json:"build,omitempty"`
}

type PortableArtifactBuild struct {
	ContextArchive []byte            `json:"contextArchive"`
	ContextDigest  string            `json:"contextDigest"`
	RecipePath     string            `json:"recipePath"`
	RecipeDigest   string            `json:"recipeDigest"`
	Arguments      map[string]string `json:"arguments,omitempty"`
}

var portableDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var portablePlatform = regexp.MustCompile(`^linux/(amd64|arm64)$`)
var portableArgument = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (o ArtifactOrigin) Validate() error {
	if o.ExpectedDigest != "" && !portableDigest.MatchString(o.ExpectedDigest) {
		return fmt.Errorf("artifact origin expectedDigest must be SHA-256")
	}
	if !portablePlatform.MatchString(o.Platform) {
		return fmt.Errorf("artifact origin requires an explicit supported platform (linux/amd64 or linux/arm64)")
	}
	if (o.Image == "") == (o.Build == nil) {
		return fmt.Errorf("artifact origin requires exactly one of image or build")
	}
	if o.Image != "" {
		parts := strings.Split(o.Image, "@")
		if len(parts) != 2 || parts[0] == "" || !portableDigest.MatchString(parts[1]) || strings.ContainsAny(o.Image, " \t\r\n") || strings.Contains(parts[0], "://") || strings.HasPrefix(parts[0], "-") {
			return fmt.Errorf("artifact origin image must use an immutable registry reference@sha256 digest")
		}
		return nil
	}
	b := o.Build
	if len(b.ContextArchive) == 0 || len(b.ContextArchive) > MaxPortableContextBytes || !portableDigest.MatchString(b.ContextDigest) || !portableDigest.MatchString(b.RecipeDigest) {
		return fmt.Errorf("portable build requires a bounded context archive and SHA-256 context/recipe digests")
	}
	if b.RecipePath == "" || path.IsAbs(b.RecipePath) || path.Clean(b.RecipePath) != b.RecipePath || b.RecipePath == ".." || strings.HasPrefix(b.RecipePath, "../") || strings.Contains(b.RecipePath, "\\") {
		return fmt.Errorf("portable build recipe must be a relative context path")
	}
	for key := range b.Arguments {
		upper := strings.ToUpper(key)
		if !portableArgument.MatchString(key) || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "KEY") {
			return fmt.Errorf("portable build argument %q is invalid or contains credentials; configure credentials in the destination", key)
		}
	}
	return nil
}
