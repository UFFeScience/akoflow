package utils

import "os"

func ReadFile(filePath string) string {
	// read file
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
		return ""
	}

	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		panic(err)
		return ""
	}

	bs := make([]byte, stat.Size())
	_, err = file.Read(bs)
	if err != nil {
		panic(err)
		return ""
	}

	return string(bs)
}
