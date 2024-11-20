package utils_delete_file

import "os"

type UtilsDeleteFile struct {
}

func New() *UtilsDeleteFile {
	return &UtilsDeleteFile{}
}

func (u *UtilsDeleteFile) DeleteFile(filePath string) error {
	// delete file
	err := os.Remove(filePath)
	if err != nil {
		return err
	}
	return nil
}
