package file_spec_parser_service

import (
	"encoding/json"
	"regexp"
	"strings"
)

type FileSpecParserService struct {
}

func New() FileSpecParserService {
	return FileSpecParserService{}
}

type DiskSpec struct {
	FileSystem     string
	Size           string
	Used           string
	Available      string
	UsedPercentage string
	MountedOn      string
}

func (s *FileSpecParserService) Parse(rawText string) string {

	diskSpecs := []DiskSpec{}
	indexLine := 0
	for _, line := range strings.Split(rawText, "\n") {
		if indexLine == 0 {
			indexLine++
			continue
		}

		diskSpec := s.parseLine(line)
		if diskSpec == nil {
			continue
		}

		diskSpecs = append(diskSpecs, *diskSpec)
	}

	diskSpecsString, _ := json.Marshal(diskSpecs)
	return string(diskSpecsString)
}

func (s *FileSpecParserService) parseLine(line string) *DiskSpec {
	// split line
	var re = regexp.MustCompile(`(.*?)(\s+|$)`)

	matches := re.FindAllString(line, -1)

	if len(matches) < 6 {
		return nil
	}

	// trim spaces
	fileSystem := strings.TrimSpace(matches[0])
	size := strings.TrimSpace(matches[1])
	used := strings.TrimSpace(matches[2])
	available := strings.TrimSpace(matches[3])
	usedPercentage := strings.TrimSpace(matches[4])
	mountPoint := strings.TrimSpace(matches[5])

	// parse line
	return &DiskSpec{
		FileSystem:     fileSystem,
		Size:           size,
		Used:           used,
		Available:      available,
		UsedPercentage: usedPercentage,
		MountedOn:      mountPoint,
	}
}
