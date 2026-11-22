package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	daemonLogRetain = 28 * 24 * time.Hour
	daemonStdLayout = "2006/01/02 15:04:05"
	daemonArgLayout = "2006-01-02-15-04"
)

func GetDaemonLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		from, to, err := daemonRange(c.Query("from"), c.Query("to"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		path := filesystem.DaemonLogPath
		if !go_pkg_filesystem_reader.Exists(path) {
			c.JSON(http.StatusOK, gin.H{"content": ""})
			return
		}
		content, err := go_pkg_filesystem.ReadText(path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		limit, err := strconv.Atoi(c.DefaultQuery("limit", "0"))
		if err != nil || limit < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": utilsmsg.New(utilsmsg.Invalid, "limit", "must be a non-negative integer").Error()})
			return
		}
		before, err := strconv.Atoi(c.DefaultQuery("before", "-1"))
		if err != nil || before < -1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": utilsmsg.New(utilsmsg.Invalid, "before", "must be a non-negative integer").Error()})
			return
		}
		keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
		level := strings.ToUpper(strings.TrimSpace(c.Query("level")))
		lines := recentDaemonLines(content, from, to, keyword, level)
		end := len(lines)
		if before >= 0 && before < end {
			end = before
		}
		start := 0
		if limit > 0 && end > limit {
			start = end - limit
		}
		text := ""
		if start < end {
			text = strings.Join(lines[start:end], "\n") + "\n"
		}
		c.JSON(http.StatusOK, gin.H{"content": text, "start": start, "more": start > 0})
	}
}

func daemonRange(fromArg, toArg string) (time.Time, time.Time, error) {
	from := time.Now().Add(-daemonLogRetain)
	var to time.Time

	if arg := strings.TrimSpace(fromArg); arg != "" {
		at, err := time.ParseInLocation(daemonArgLayout, arg, time.Local)
		if err != nil {
			return from, to, utilsmsg.New(utilsmsg.Invalid, "from", "must be yyyy-MM-dd-HH-mm")
		}
		from = at
	}
	if arg := strings.TrimSpace(toArg); arg != "" {
		at, err := time.ParseInLocation(daemonArgLayout, arg, time.Local)
		if err != nil {
			return from, to, utilsmsg.New(utilsmsg.Invalid, "to", "must be yyyy-MM-dd-HH-mm")
		}
		to = at
	}
	return from, to, nil
}

func recentDaemonLines(content string, from, to time.Time, keyword, level string) []string {
	var lines []string
	keep := false
	for line := range strings.SplitSeq(strings.TrimRight(content, "\n"), "\n") {
		at, timed := daemonLineTime(line)
		if timed {
			keep = !at.Before(from) &&
				(to.IsZero() || at.Before(to)) &&
				(keyword == "" || strings.Contains(strings.ToLower(line), keyword)) &&
				(level == "" || daemonLineLevel(line) == level)
		}
		if keep && (timed || level == "") {
			lines = append(lines, line)
		}
	}
	return lines
}

func daemonLineLevel(line string) string {
	if _, rest, ok := strings.Cut(line, " level="); ok && strings.HasPrefix(line, "time=") {
		field, _, _ := strings.Cut(rest, " ")
		return field
	}
	if len(line) <= len(daemonStdLayout) {
		return ""
	}
	field, _, _ := strings.Cut(strings.TrimSpace(line[len(daemonStdLayout):]), " ")
	return field
}

func daemonLineTime(line string) (time.Time, bool) {
	if rest, ok := strings.CutPrefix(line, "time="); ok {
		field, _, _ := strings.Cut(rest, " ")
		at, err := time.Parse(time.RFC3339, field)
		return at, err == nil
	}
	if len(line) < len(daemonStdLayout) {
		return time.Time{}, false
	}
	at, err := time.ParseInLocation(daemonStdLayout, line[:len(daemonStdLayout)], time.Local)
	return at, err == nil
}
