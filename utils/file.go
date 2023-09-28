package utils

import (
	"bytes"
	"fmt"
	"os"
)

func ReplaceInFile(file string, search string, replace string) {
	input, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	output := bytes.Replace(input, []byte(search), []byte(replace), -1)

	if err = os.WriteFile(file, output, 0666); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
