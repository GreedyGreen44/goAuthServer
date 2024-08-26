package main

import (
	"bufio"
	"errors"
	"os"
)

// reads database paramemters from file
func ReadParamsFromFile(fileName string) ([]string, error) {
	var paramSlice []string

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		paramSlice = append(paramSlice, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(paramSlice) != 4 {
		err := errors.New("incorrect file structure")
		return nil, err
	}
	return paramSlice, nil
}
