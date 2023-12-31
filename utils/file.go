package utils

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
)

func FileContains(file string, search string) bool {
	input, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	return bytes.Contains(input, []byte(search))
}

func ReplaceInFile(file string, search string, replace string) bool {
	input, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	contains := bytes.Contains(input, []byte(search))
	if !contains {
		return false
	}
	output := bytes.Replace(input, []byte(search), []byte(replace), -1)

	if err = os.WriteFile(file, output, 0666); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return contains
}

func ReplaceLineInFile(file string, search string, replace string) {
	input, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, search) {
			lines[i] = replace
		}
	}
	output := strings.Join(lines, "\n")
	err = os.WriteFile(file, []byte(output), 0644)
	if err != nil {
		log.Fatalln(err)
	}
}
