package intelrdt

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type lineType int

const (
	l3line lineType = iota
	l3dataline
	l3codeline
	mbline
	unknown
)

type parsedLine struct {
	prefix lineType
	tokens map[string]string
}

func getLineType(line string) lineType {
	if strings.HasPrefix(line, "MB:") {
		return mbline
	} else if strings.HasPrefix(line, "L3:") {
		return l3line
	} else if strings.HasPrefix(line, "L3DATA:") {
		return l3dataline
	} else if strings.HasPrefix(line, "L3CODE:") {
		return l3codeline
	}
	return unknown
}

func parseLine(lineType lineType, line string) (map[string]string, error) {
	switch lineType {
	case l3line:
		line = strings.TrimPrefix(line, "L3:")
	case l3codeline:
		line = strings.TrimPrefix(line, "L3CODE:")
	case l3dataline:
		line = strings.TrimPrefix(line, "L3DATA:")
	case mbline:
		line = strings.TrimPrefix(line, "MB:")
	}

	tokenMap := make(map[string]string)

	// Split the line on ';'.
	tokens := strings.Split(line, ";")

	for _, token := range tokens {
		// Check for empty tokens (resulting from "; ;" etc).
		if strings.TrimSpace(token) == "" {
			continue
		}

		// Split the token on '='.
		values := strings.Split(token, "=")
		if len(values) != 2 {
			return nil, fmt.Errorf("error parsing token '%s': wrong len", token)
		}

		// Remove spaces around the token parts.
		key := strings.TrimSpace(values[0])
		value := strings.TrimSpace(values[1])

		// Need to have values on both sides of '='.
		if key == "" {
			return nil, fmt.Errorf("error parsing token '%s': empty key", token)
		}
		if value == "" {
			return nil, fmt.Errorf("error parsing token '%s': empty value", token)
		}

		if lineType == mbline {
			tokenMap[key] = value
		} else {
			// Decode from hex and encode back. This removes the leading zeros and
			// validates the value.
			hexNum, err := strconv.ParseInt(value, 16, 64)
			if err != nil {
				return nil, err
			}
			tokenMap[key] = fmt.Sprintf("%x", hexNum)
		}
	}

	return tokenMap, nil
}

func parseLinesToTokenMaps(lines []string) ([]parsedLine, error) {
	parsedLines := make([]parsedLine, 0)

	for _, line := range lines {
		lineType := getLineType(line)

		if lineType == unknown {
			// Unknown line type. This is probably some resource in the schemata
			// file. Since we are looking for a match for the "L3:", "L3DATA:",
			// "L3CODE:", and "MB:" lines, we will skip this.
			continue
		}

		tokenMap, err := parseLine(lineType, line)
		if err != nil {
			return nil, err
		}

		parsedLines = append(parsedLines, parsedLine{
			prefix: lineType,
			tokens: tokenMap,
		})
	}

	return parsedLines, nil
}

func compareLine(a, b parsedLine) bool {
	// Check prefixes and that both maps have the same number of keys.
	if a.prefix != b.prefix || len(a.tokens) != len(b.tokens) {
		return false
	}

	// Check that both maps have the same keys and values.
	for key, aVal := range a.tokens {
		bVal, ok := b.tokens[key]
		if !ok || aVal != bVal {
			return false
		}
	}

	return true
}

func compareParsedLines(aLines, bLines []parsedLine) bool {
	// Each line in aLines must match to at least one match in bLines
	// (and vice versa, because aLines might have duplicates).

	for _, a := range aLines {
		found := false
		for _, b := range bLines {
			if compareLine(a, b) {
				found = true
				break
			}
		}
		if !found {
			// No match for a in bLines.
			return false
		}
	}

	for _, b := range bLines {
		found := false
		for _, a := range aLines {
			if compareLine(b, a) {
				found = true
				break
			}
		}
		if !found {
			// No match for b in aLines.
			return false
		}
	}

	return true
}

func checkExistingSchemata(path, schemata string) error {
	// Read the existing schemata to a string.
	existingSchemata, err := os.ReadFile(filepath.Join(path, "schemata"))
	if err != nil {
		return err
	}

	return checkSchemataMatch(string(existingSchemata), schemata)
}

func removeEmptyLines(lines []string) []string {
	removedLines := 0

	for i, line := range lines {
		if len(line) == 0 {
			removedLines++
			continue
		}
		lines[i-removedLines] = lines[i]
	}

	// Remove empty space from the end and return the lines.
	return lines[:len(lines)-removedLines]
}

func checkSchemataMatch(existingSchemata, newSchemata string) error {
	// Split both schemata to lines and  remove empty lines.
	existingSchemataLines := removeEmptyLines(strings.Split(existingSchemata, "\n"))
	newSchemataLines := removeEmptyLines(strings.Split(newSchemata, "\n"))

	// Parse the lines to token maps.
	existingSchemataParsedLines, err := parseLinesToTokenMaps(existingSchemataLines)
	if err != nil {
		return err
	}
	newSchemataParsedLines, err := parseLinesToTokenMaps(newSchemataLines)
	if err != nil {
		return err
	}

	// Compare the map lists with each other.
	matches := compareParsedLines(newSchemataParsedLines, existingSchemataParsedLines)
	if !matches {
		return fmt.Errorf("lines don't match")
	}

	return nil
}
