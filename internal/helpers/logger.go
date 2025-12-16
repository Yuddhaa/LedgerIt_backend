package helpers

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// Logger is the slog.Logger.
// Based on our main.go, this logger *only* writes compact JSON to "log.log".
var Logger *slog.Logger

// logIndentedToConsole is a helper to manually create and print
// indented JSON to the console (os.Stdout).
func logIndentedToConsole(level, calling, msg string, data ...any) {
	// Create a map to hold all log attributes
	temp := make(map[string]any)
	temp["level"] = level
	temp["msg"] = msg

	// Add the 'source' attribute if provided
	if calling != "" {
		temp["source"] = calling
	}

	// Safely parse the key-value pairs from data
	n := len(data)
	for i := 0; i < n; i += 2 {
		// Ensure we don't go out of bounds
		if i+1 >= n {
			break // Or log a malformed data error
		}

		// Ensure the key is a string
		key, ok := data[i].(string)
		if !ok {
			key = fmt.Sprintf("invalid_key_%d", i)
		}
		temp[key] = data[i+1]
	}

	// Marshal the map with indentation
	// Using "  " for indent is more standard than "   "
	prettyJSON, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		// Fallback if marshaling fails
		fmt.Printf("{\"level\":\"ERROR\", \"msg\":\"failed to marshal log\", \"error\":\"%v\"}\n", err.Error())
		return
	}

	// Print the final indented JSON string to the console
	fmt.Printf("\n%s\n", string(prettyJSON))
}

// LogError logs an ERROR message.
// It writes compact JSON to the file AND indented JSON to the console.
func LogError(calling, msg string, data ...any) {
	tempData := append(data, "source", calling)
	// 1. Log compact JSON to log.log
	Logger.Error(msg, tempData...)

	// 2. Log indented JSON to the console
	logIndentedToConsole("ERROR", calling, msg, data...)
}

// LogInfo logs an INFO message.
// It writes compact JSON to the file AND indented JSON to the console.
func LogInfo(calling, msg string, data ...any) {
	tempData := append(data, "source", calling)
	// 1. Log compact JSON to log.log
	Logger.Info(msg, tempData...)

	// 2. Log indented JSON to the console
	logIndentedToConsole("INFO", calling, msg, data...)
}

// PrintJson is a utility function to pretty-print any data structure.
func PrintJson(msg string, payload any) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Printf("\n%s\nError marshaling JSON: %v\n", msg, err.Error())
		return
	}

	if msg == "" {
		fmt.Printf("\n%s\n", string(data))
	} else {
		fmt.Printf("\n%s:\n%s\n", msg, string(data))
	}
}
