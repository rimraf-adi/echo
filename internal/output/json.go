package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON formats v as indented JSON and writes to stdout
func PrintJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// PrintErrorJSON formats err as JSON and writes to stderr
func PrintErrorJSON(err error) {
	errObj := map[string]string{
		"error": err.Error(),
	}
	data, _ := json.MarshalIndent(errObj, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
}
