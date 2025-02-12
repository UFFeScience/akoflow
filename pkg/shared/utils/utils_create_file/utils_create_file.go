package utils_create_file

import "os"

type UtilsCreateFile struct {
}

func New() *UtilsCreateFile {
	return &UtilsCreateFile{}
}

func (u *UtilsCreateFile) CreateFile(filePath string, content string) {
	// create file
	file, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	_, err = file.Write([]byte(content))
	if err != nil {
		panic(err)
	}
}

func (u *UtilsCreateFile) CreateTempFile(content string) string {
	// create temporary file
	file, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}

	defer file.Close()

	_, err = file.Write([]byte(content))
	if err != nil {
		panic(err)
	}

	return file.Name()
}
