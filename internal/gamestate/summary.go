package gamestate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func SnapshotLines(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var data interface{}
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return []string{"(invalid JSON)"}
	}

	lines := make([]string, 0, 64)
	walkValue(data, "", &lines)
	return lines
}

func SnapshotString(content string) string {
	lines := SnapshotLines(content)
	return strings.Join(lines, "\n")
}

func walkValue(value interface{}, path string, lines *[]string) {
	switch v := value.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			appendLine(path, "{}", lines)
			return
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			walkValue(v[key], joinPath(path, key), lines)
		}
	case []interface{}:
		if len(v) == 0 {
			appendLine(path, "[]", lines)
			return
		}
		for i, item := range v {
			walkValue(item, fmt.Sprintf("%s[%d]", path, i), lines)
		}
	default:
		appendLine(path, formatPrimitive(v), lines)
	}
}

func appendLine(path, value string, lines *[]string) {
	if path == "" {
		*lines = append(*lines, value)
		return
	}
	*lines = append(*lines, path+": "+value)
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func formatPrimitive(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case bool:
		return strconv.FormatBool(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
