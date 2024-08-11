package manipulation_files_service

import (
	"os"
	"path/filepath"
)

type ManipulationFilesService struct {
}

func New() *ManipulationFilesService {
	return &ManipulationFilesService{}
}

func (s *ManipulationFilesService) ListAllFilesInDir(dir string) []string {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {

		if info.IsDir() {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil
	}
	return files
}
