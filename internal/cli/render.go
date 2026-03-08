package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type responseMetadata struct {
	Count      *int     `json:"count,omitempty"`
	Truncated  bool     `json:"truncated,omitempty"`
	Command    string   `json:"command,omitempty"`
	NextAction string   `json:"next_action,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type responseEnvelope struct {
	Status   string            `json:"status"`
	Data     any               `json:"data"`
	Metadata *responseMetadata `json:"metadata,omitempty"`
}

var collectionKeys = []string{"items", "data", "results", "result", "traces", "queryHistory", "slos", "serviceAccounts"}

func withCommandMetadata(meta *responseMetadata, command string) *responseMetadata {
	if meta == nil {
		meta = &responseMetadata{}
	}
	meta.Command = command
	return meta
}

func collectionMetadata(command string, payload any, limit int, nextAction string) *responseMetadata {
	count, ok := inferMetadataCount(payload)
	if !ok && command == "" && nextAction == "" && limit <= 0 {
		return nil
	}
	meta := &responseMetadata{Command: command}
	if ok {
		meta.Count = &count
	}
	if limit > 0 && ok && count >= limit {
		meta.Truncated = true
		if strings.TrimSpace(nextAction) != "" {
			meta.NextAction = nextAction
		}
	}
	return meta
}

func inferMetadataCount(payload any) (int, bool) {
	switch value := payload.(type) {
	case []any:
		return len(value), true
	case map[string]any:
		for _, key := range collectionKeys {
			candidate, ok := value[key]
			if !ok {
				continue
			}
			switch typed := candidate.(type) {
			case []any:
				return len(typed), true
			case map[string]any:
				if nested, ok := inferMetadataCount(typed); ok {
					return nested, true
				}
			}
		}
	}
	return 0, false
}

func renderTable(out io.Writer, payload any) error {
	rows := rowsForTable(payload)
	if len(rows) == 0 {
		_, err := fmt.Fprintln(out, "No rows")
		return err
	}
	headers := tableHeaders(rows)
	if _, err := fmt.Fprintln(out, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		values := make([]string, 0, len(headers))
		for _, header := range headers {
			values = append(values, tableCell(row[header]))
		}
		if _, err := fmt.Fprintln(out, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func rowsForTable(payload any) []map[string]any {
	switch value := payload.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, flattenTableRow(row, ""))
			}
		}
		return rows
	case map[string]any:
		if count, ok := inferMetadataCount(value); ok && count > 0 {
			for _, key := range collectionKeys {
				candidate, ok := value[key]
				if !ok {
					continue
				}
				if rows := rowsForTable(candidate); len(rows) > 0 {
					return rows
				}
			}
		}
		return []map[string]any{flattenTableRow(value, "")}
	default:
		return nil
	}
}

func flattenTableRow(row map[string]any, prefix string) map[string]any {
	flat := map[string]any{}
	for key, value := range row {
		name := key
		if prefix != "" {
			name = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			for childKey, childValue := range flattenTableRow(nested, name) {
				flat[childKey] = childValue
			}
			continue
		}
		flat[name] = value
	}
	return flat
}

func tableHeaders(rows []map[string]any) []string {
	set := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			set[key] = struct{}{}
		}
	}
	headers := make([]string, 0, len(set))
	for key := range set {
		headers = append(headers, key)
	}
	sort.Strings(headers)
	return headers
}

func tableCell(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64, float32, int, int64, int32, uint, uint64:
		return fmt.Sprint(typed)
	case []any:
		return fmt.Sprintf("[%d items]", len(typed))
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}
