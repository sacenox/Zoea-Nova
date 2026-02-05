package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// Tree box-drawing characters
	treeEdge  = "├─"
	treeLast  = "└─"
	treeVert  = "│ "
	treeSpace = "  "

	// Truncation limits
	arrayTruncateThreshold = 6 // Show first 3 and last 3 if array has more than this
	arrayShowFirst         = 3
	arrayShowLast          = 3
)

// renderJSONTree renders JSON as a Unicode tree structure with smart truncation.
// If verbose is true, all items are shown. Otherwise, arrays with more than 6 items
// show first 3, "[x more]", and last 3.
func renderJSONTree(jsonStr string, verbose bool) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	var lines []string
	renderValue(data, "", true, verbose, &lines)
	return strings.Join(lines, "\n"), nil
}

// renderValue recursively renders a JSON value as tree lines
func renderValue(value interface{}, prefix string, isLast bool, verbose bool, lines *[]string) {
	switch v := value.(type) {
	case map[string]interface{}:
		renderObject(v, prefix, isLast, verbose, lines)
	case []interface{}:
		renderArray(v, prefix, isLast, verbose, lines)
	default:
		// Primitive value
		*lines = append(*lines, prefix+fmt.Sprintf("%v", v))
	}
}

// renderObject renders a JSON object
func renderObject(obj map[string]interface{}, prefix string, isLast bool, verbose bool, lines *[]string) {
	if len(obj) == 0 {
		*lines = append(*lines, prefix+"{}")
		return
	}

	*lines = append(*lines, prefix+"{")

	// Get keys in deterministic order (sorted)
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	// Note: Not sorting to preserve JSON order, but could add sort.Strings(keys) here

	for i, key := range keys {
		isLastField := i == len(keys)-1
		fieldPrefix := prefix
		if isLast {
			fieldPrefix += treeSpace
		} else {
			fieldPrefix += treeVert
		}

		var connector string
		if isLastField {
			connector = treeLast
		} else {
			connector = treeEdge
		}

		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange for keys
		valuePrefix := fieldPrefix + connector + keyStyle.Render(fmt.Sprintf("%q: ", key))

		val := obj[key]
		switch v := val.(type) {
		case map[string]interface{}:
			*lines = append(*lines, valuePrefix)
			renderObject(v, fieldPrefix+treeSpace, isLastField, verbose, lines)
		case []interface{}:
			*lines = append(*lines, valuePrefix)
			renderArray(v, fieldPrefix+treeSpace, isLastField, verbose, lines)
		default:
			*lines = append(*lines, valuePrefix+fmt.Sprintf("%v", v))
		}
	}

	closingPrefix := prefix
	if !isLast {
		closingPrefix += treeVert
	} else {
		closingPrefix += treeSpace
	}
	*lines = append(*lines, closingPrefix+"}")
}

// renderArray renders a JSON array with smart truncation
func renderArray(arr []interface{}, prefix string, isLast bool, verbose bool, lines *[]string) {
	if len(arr) == 0 {
		*lines = append(*lines, prefix+"[]")
		return
	}

	*lines = append(*lines, prefix+"[")

	shouldTruncate := !verbose && len(arr) > arrayTruncateThreshold

	itemsToShow := len(arr)
	if shouldTruncate {
		itemsToShow = arrayShowFirst + arrayShowLast
	}

	showItems := make([]int, 0, itemsToShow)
	if shouldTruncate {
		// First 3 items
		for i := 0; i < arrayShowFirst; i++ {
			showItems = append(showItems, i)
		}
		// Last 3 items
		for i := len(arr) - arrayShowLast; i < len(arr); i++ {
			showItems = append(showItems, i)
		}
	} else {
		// Show all items
		for i := 0; i < len(arr); i++ {
			showItems = append(showItems, i)
		}
	}

	truncatedCount := len(arr) - len(showItems)

	for idx, i := range showItems {
		isLastItem := idx == len(showItems)-1

		// Insert truncation indicator between first and last items
		if shouldTruncate && idx == arrayShowFirst {
			truncPrefix := prefix
			if isLast {
				truncPrefix += treeSpace
			} else {
				truncPrefix += treeVert
			}
			dimmedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			*lines = append(*lines, truncPrefix+treeEdge+dimmedStyle.Render(fmt.Sprintf("[%d more]", truncatedCount)))
		}

		itemPrefix := prefix
		if isLast {
			itemPrefix += treeSpace
		} else {
			itemPrefix += treeVert
		}

		var connector string
		if isLastItem && !shouldTruncate {
			connector = treeLast
		} else {
			connector = treeEdge
		}

		indexStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim gray for indices
		itemLine := itemPrefix + connector + indexStyle.Render(fmt.Sprintf("[%d] ", i))

		val := arr[i]
		switch v := val.(type) {
		case map[string]interface{}:
			*lines = append(*lines, itemLine)
			renderObject(v, itemPrefix+treeSpace, isLastItem, verbose, lines)
		case []interface{}:
			*lines = append(*lines, itemLine)
			renderArray(v, itemPrefix+treeSpace, isLastItem, verbose, lines)
		default:
			*lines = append(*lines, itemLine+fmt.Sprintf("%v", v))
		}
	}

	closingPrefix := prefix
	if !isLast {
		closingPrefix += treeVert
	} else {
		closingPrefix += treeSpace
	}
	*lines = append(*lines, closingPrefix+"]")
}
