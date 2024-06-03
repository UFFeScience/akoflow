package file_list_parser_service

import (
	"encoding/json"
	"strings"
)

type FileListParserService struct {
}

func New() *FileListParserService {
	return &FileListParserService{}
}

type FileDisk struct {
	Permissions  string
	Owner        string
	Group        string
	Size         string
	LastModified string
	Name         string
	Path         string
}

func (s *FileListParserService) parseFileList(input string) []FileDisk {

	filesDisk := []FileDisk{}
	lines := strings.Split(input, "\n")

	var lastPath string = "./"
	for _, line := range lines {

		if strings.Contains(line, "./") && strings.Contains(line, ":") {
			lastPath = s.getPath(line)
		}

		files := strings.Split(line, " ")
		filesTrim := []string{}
		for _, file := range files {
			if file == "" {
				continue
			}
			filesTrim = append(filesTrim, strings.TrimSpace(file))
		}

		if len(filesTrim) != 9 {
			continue
		}

		fileDisk := FileDisk{
			Permissions:  strings.TrimSpace(filesTrim[0]),
			Owner:        strings.TrimSpace(filesTrim[2]),
			Group:        strings.TrimSpace(filesTrim[3]),
			Size:         strings.TrimSpace(filesTrim[4]),
			LastModified: strings.TrimSpace(filesTrim[5]+" "+filesTrim[6]) + " " + filesTrim[7],
			Name:         strings.TrimSpace(filesTrim[8]),
			Path:         lastPath,
		}

		filesDisk = append(filesDisk, fileDisk)

	}

	return filesDisk
}

func (s *FileListParserService) getPath(line string) string {
	if strings.Contains(line, "./") && strings.Contains(line, ":") {
		return strings.Split(line, ":")[0]
	}
	return ""
}

func (s *FileListParserService) Parse(rawText string) string {
	filesDisk := s.parseFileList(rawText)

	jsonFilesDisk, _ := json.Marshal(filesDisk)

	return string(jsonFilesDisk)

}
