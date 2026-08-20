package cli

import (
	"sort"
	"strings"
)

func commandDistance(left, right string) int {
	left, right = strings.ToLower(left), strings.ToLower(right)
	rows := make([]int, len(right)+1)
	for j := range rows {
		rows[j] = j
	}
	for i, a := range left {
		previous := rows[0]
		rows[0] = i + 1
		for j, b := range right {
			saved := rows[j+1]
			cost := 0
			if a != b {
				cost = 1
			}
			deletion, insertion, replacement := rows[j+1]+1, rows[j]+1, previous+cost
			rows[j+1] = minInt(deletion, minInt(insertion, replacement))
			previous = saved
		}
	}
	return rows[len(right)]
}
func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
func workspaceSuggestions(items []map[string]any, requested string) []string {
	result := []string{}
	for _, item := range items {
		handle := stringField(item, "handle")
		if handle != "" && commandDistance(requested, handle) <= 3 {
			result = append(result, handle)
		}
	}
	sort.Strings(result)
	if len(result) > 3 {
		result = result[:3]
	}
	return result
}
