package helpers

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// Logger is the slog.Logger.
// Based on our main.go, this logger *only* writes compact JSON to "log.log".
var Logger *slog.Logger

// LogStreamHook is a function that will be called for every log entry.
// We will assign the WebSocket broadcaster to this later.
var LogStreamHook func(entry map[string]any)

// logIndentedToConsole is a helper to manually create and print...
func logIndentedToConsole(level, calling, msg string, data ...any) {
	// Create a map to hold all log attributes
	temp := make(map[string]any)
	temp["level"] = level
	temp["msg"] = msg

	if calling != "" {
		temp["source"] = calling
	}

	// Safely parse the key-value pairs from data
	n := len(data)
	for i := 0; i < n; i += 2 {
		if i+1 >= n {
			break
		}
		key, ok := data[i].(string)
		if !ok {
			key = fmt.Sprintf("invalid_key_%d", i)
		}
		temp[key] = data[i+1]
	}

	// 2. ADD THIS BLOCK
	// If a hook is registered (WebSocket is running), send the data there.
	if LogStreamHook != nil {
		// We pass a copy or the map directly.
		// Since we marshal immediately in the hook, passing map is fine.
		LogStreamHook(temp)
	}

	// Marshal the map with indentation for Console
	prettyJSON, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		fmt.Printf("{\"level\":\"ERROR\", \"msg\":\"failed to marshal log\", \"error\":\"%v\"}\n", err.Error())
		return
	}

	fmt.Printf("\n%s\n", string(prettyJSON))
}

// ... LogError and LogInfo remain exactly the same ...
func LogError(calling, msg string, data ...any) {
	tempData := append(data, "source", calling)
	Logger.Error(msg, tempData...)
	logIndentedToConsole("ERROR", calling, msg, data...)
}

func LogInfo(calling, msg string, data ...any) {
	tempData := append(data, "source", calling)
	Logger.Info(msg, tempData...)
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
