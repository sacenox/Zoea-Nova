package tui

import (
	"encoding/json"
	"fmt"
	"sort"
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
// maxWidth constrains the width of rendered lines (0 = no limit).
func renderJSONTree(jsonStr string, verbose bool, maxWidth int) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	var lines []string
	renderValue(data, "", true, verbose, maxWidth, &lines)
	return strings.Join(lines, "\n"), nil
}

// renderValue recursively renders a JSON value as tree lines
func renderValue(value interface{}, prefix string, isLast bool, verbose bool, maxWidth int, lines *[]string) {
	switch v := value.(type) {
	case map[string]interface{}:
		renderObject(v, prefix, isLast, verbose, maxWidth, lines)
	case []interface{}:
		renderArray(v, prefix, isLast, verbose, maxWidth, lines)
	default:
		// Primitive value - wrap if needed
		valueStr := fmt.Sprintf("%v", v)
		if maxWidth > 0 {
			prefixWidth := lipgloss.Width(prefix)
			availableWidth := maxWidth - prefixWidth
			if availableWidth < 10 {
				availableWidth = 10
			}
			if lipgloss.Width(valueStr) > availableWidth {
				// Truncate long values
				valueStr = truncateToWidth(valueStr, availableWidth-3) + "..."
			}
		}
		*lines = append(*lines, prefix+valueStr)
	}
}

// renderObject renders a JSON object
func renderObject(obj map[string]interface{}, prefix string, isLast bool, verbose bool, maxWidth int, lines *[]string) {
	if len(obj) == 0 {
		// Empty object - show nothing (or could show "(empty)")
		return
	}

	// No opening brace - just render content directly

	// Get keys in deterministic order (sorted)
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

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
			// Nested object - no braces
			if len(v) == 0 {
				// Empty nested object - show label only
				*lines = append(*lines, valuePrefix+"(empty)")
			} else {
				*lines = append(*lines, valuePrefix)
				childPrefix := fieldPrefix + treeSpace
				renderObjectContent(v, childPrefix, isLastField, verbose, maxWidth, lines)
			}
		case []interface{}:
			// Nested array - no brackets
			if len(v) == 0 {
				// Empty array - show label only
				*lines = append(*lines, valuePrefix+"(empty)")
			} else {
				*lines = append(*lines, valuePrefix)
				childPrefix := fieldPrefix + treeSpace
				renderArrayContent(v, childPrefix, isLastField, verbose, maxWidth, lines)
			}
		default:
			// Primitive value - wrap if needed
			valueStr := fmt.Sprintf("%v", v)
			if maxWidth > 0 {
				prefixWidth := lipgloss.Width(valuePrefix)
				availableWidth := maxWidth - prefixWidth
				if availableWidth < 10 {
					availableWidth = 10
				}
				if lipgloss.Width(valueStr) > availableWidth {
					// Truncate long values
					valueStr = truncateToWidth(valueStr, availableWidth-3) + "..."
				}
			}
			*lines = append(*lines, valuePrefix+valueStr)
		}
	}

	// No closing brace needed
}

// renderObjectContent renders only the content of an object (no outer braces)
// Used when inlining nested objects
func renderObjectContent(obj map[string]interface{}, prefix string, isLast bool, verbose bool, maxWidth int, lines *[]string) {
	// Get keys in deterministic order (sorted)
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, key := range keys {
		isLastField := i == len(keys)-1

		var connector string
		if isLastField {
			connector = treeLast
		} else {
			connector = treeEdge
		}

		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange for keys
		valuePrefix := prefix + connector + keyStyle.Render(fmt.Sprintf("%q: ", key))

		val := obj[key]
		switch v := val.(type) {
		case map[string]interface{}:
			// Inline nested object - no braces
			if len(v) == 0 {
				*lines = append(*lines, valuePrefix+"(empty)")
			} else {
				*lines = append(*lines, valuePrefix)
				childPrefix := prefix
				if !isLastField {
					childPrefix += treeVert
				} else {
					childPrefix += treeSpace
				}
				renderObjectContent(v, childPrefix, isLastField, verbose, maxWidth, lines)
			}
		case []interface{}:
			// Inline nested array - no brackets
			if len(v) == 0 {
				*lines = append(*lines, valuePrefix+"(empty)")
			} else {
				*lines = append(*lines, valuePrefix)
				childPrefix := prefix
				if !isLastField {
					childPrefix += treeVert
				} else {
					childPrefix += treeSpace
				}
				renderArrayContent(v, childPrefix, isLastField, verbose, maxWidth, lines)
			}
		default:
			// Primitive value - wrap if needed
			valueStr := fmt.Sprintf("%v", v)
			if maxWidth > 0 {
				prefixWidth := lipgloss.Width(valuePrefix)
				availableWidth := maxWidth - prefixWidth
				if availableWidth < 10 {
					availableWidth = 10
				}
				if lipgloss.Width(valueStr) > availableWidth {
					// Truncate long values
					valueStr = truncateToWidth(valueStr, availableWidth-3) + "..."
				}
			}
			*lines = append(*lines, valuePrefix+valueStr)
		}
	}
}

// renderArrayContent renders only the content of an array (no outer brackets)
// Used when inlining nested arrays
func renderArrayContent(arr []interface{}, prefix string, isLast bool, verbose bool, maxWidth int, lines *[]string) {
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
			dimmedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			*lines = append(*lines, prefix+treeEdge+dimmedStyle.Render(fmt.Sprintf("[%d more]", truncatedCount)))
		}

		var connector string
		if isLastItem {
			connector = treeLast
		} else {
			connector = treeEdge
		}

		indexStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim gray for indices
		itemLine := prefix + connector + indexStyle.Render(fmt.Sprintf("[%d] ", i))

		val := arr[i]
		switch v := val.(type) {
		case map[string]interface{}:
			// Inline nested object - no braces
			if len(v) == 0 {
				*lines = append(*lines, itemLine+"(empty)")
			} else {
				*lines = append(*lines, itemLine)
				childPrefix := prefix
				if !isLastItem {
					childPrefix += treeVert
				} else {
					childPrefix += treeSpace
				}
				renderObjectContent(v, childPrefix, isLastItem, verbose, maxWidth, lines)
			}
		case []interface{}:
			// Inline nested array - no brackets
			if len(v) == 0 {
				*lines = append(*lines, itemLine+"(empty)")
			} else {
				*lines = append(*lines, itemLine)
				childPrefix := prefix
				if !isLastItem {
					childPrefix += treeVert
				} else {
					childPrefix += treeSpace
				}
				renderArrayContent(v, childPrefix, isLastItem, verbose, maxWidth, lines)
			}
		default:
			// Primitive value - wrap if needed
			valueStr := fmt.Sprintf("%v", v)
			if maxWidth > 0 {
				prefixWidth := lipgloss.Width(itemLine)
				availableWidth := maxWidth - prefixWidth
				if availableWidth < 10 {
					availableWidth = 10
				}
				if lipgloss.Width(valueStr) > availableWidth {
					// Truncate long values
					valueStr = truncateToWidth(valueStr, availableWidth-3) + "..."
				}
			}
			*lines = append(*lines, itemLine+valueStr)
		}
	}
}

// renderArray renders a JSON array with smart truncation
func renderArray(arr []interface{}, prefix string, isLast bool, verbose bool, maxWidth int, lines *[]string) {
	if len(arr) == 0 {
		// Empty array - show nothing
		return
	}

	// No opening bracket - just render content directly

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
			renderObject(v, itemPrefix+treeSpace, isLastItem, verbose, maxWidth, lines)
		case []interface{}:
			*lines = append(*lines, itemLine)
			renderArray(v, itemPrefix+treeSpace, isLastItem, verbose, maxWidth, lines)
		default:
			// Primitive value - wrap if needed
			valueStr := fmt.Sprintf("%v", v)
			if maxWidth > 0 {
				prefixWidth := lipgloss.Width(itemLine)
				availableWidth := maxWidth - prefixWidth
				if availableWidth < 10 {
					availableWidth = 10
				}
				if lipgloss.Width(valueStr) > availableWidth {
					// Truncate long values
					valueStr = truncateToWidth(valueStr, availableWidth-3) + "..."
				}
			}
			*lines = append(*lines, itemLine+valueStr)
		}
	}

	// No closing bracket needed
}
