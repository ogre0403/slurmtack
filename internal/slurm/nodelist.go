package slurm

import (
	"fmt"
	"strconv"
	"strings"
)

func expandNodeList(expr string) []string {
	if expr == "" {
		return nil
	}
	var result []string
	for _, seg := range splitTopLevel(expr) {
		result = append(result, expandSegment(seg)...)
	}
	return result
}

func splitTopLevel(expr string) []string {
	var segments []string
	depth := 0
	start := 0
	for i, ch := range expr {
		switch ch {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				segments = append(segments, expr[start:i])
				start = i + 1
			}
		}
	}
	segments = append(segments, expr[start:])
	return segments
}

func expandSegment(seg string) []string {
	bracketStart := strings.IndexByte(seg, '[')
	if bracketStart < 0 {
		return []string{seg}
	}
	bracketEnd := strings.IndexByte(seg, ']')
	if bracketEnd < 0 {
		return []string{seg}
	}
	prefix := seg[:bracketStart]
	suffix := seg[bracketEnd+1:]
	rangeExpr := seg[bracketStart+1 : bracketEnd]

	var expanded []string
	for _, part := range strings.Split(rangeExpr, ",") {
		if dashIdx := strings.IndexByte(part, '-'); dashIdx >= 0 {
			startStr := part[:dashIdx]
			endStr := part[dashIdx+1:]
			startNum, err1 := strconv.Atoi(startStr)
			endNum, err2 := strconv.Atoi(endStr)
			if err1 != nil || err2 != nil {
				expanded = append(expanded, prefix+part+suffix)
				continue
			}
			width := len(startStr)
			for n := startNum; n <= endNum; n++ {
				node := fmt.Sprintf("%s%0*d%s", prefix, width, n, suffix)
				expanded = append(expanded, node)
			}
		} else {
			expanded = append(expanded, prefix+part+suffix)
		}
	}
	return expanded
}
