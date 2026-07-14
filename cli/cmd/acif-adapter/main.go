package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 17*1024*1024)

	writer := bufio.NewWriter(w)
	for scanner.Scan() {
		response := handleLine(scanner.Bytes())
		data, err := json.Marshal(response)
		if err != nil {
			return err
		}
		if _, err := writer.Write(data); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func handleLine(line []byte) (response any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			response = errorResponse("adapter: " + fmt.Sprint(recovered))
		}
	}()

	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return errorResponse("adapter: malformed request line")
	}
	return dispatch(req)
}
