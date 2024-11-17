package utils_read_file

import "os"

type UtilsReadFile struct {
}

func New() *UtilsReadFile {
	return &UtilsReadFile{}
}

func (u *UtilsReadFile) ReadFile(filePath string) string {
	// read file
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		panic(err)
	}

	bs := make([]byte, stat.Size())
	_, err = file.Read(bs)
	if err != nil {
		panic(err)
	}

	return string(bs)
}
