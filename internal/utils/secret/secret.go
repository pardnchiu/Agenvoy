package secret

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"

	"github.com/pardnchiu/agenvoy/configs"
)

const (
	Mask       = "********"
	edgeKeep   = 3
	edgeMinLen = 9
)

type pattern struct {
	Name         string `json:"name"`
	Pattern      string `json:"pattern"`
	SkipHex      bool   `json:"skip_hex"`
	SkipCall     bool   `json:"skip_call"`
	RequireDigit bool   `json:"require_digit"`

	re    *regexp.Regexp
	group int
}

var patterns = func() []pattern {
	var list []pattern
	if err := json.Unmarshal(configs.SecretPatterns, &list); err != nil {
		panic("[Internal] jsons/secret_patterns.json: " + err.Error())
	}
	for i := range list {
		list[i].re = regexp.MustCompile(list[i].Pattern)
		list[i].group = list[i].re.SubexpIndex("secret")
	}
	return list
}()

var (
	pemBegin = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----`)
	pemEnd   = regexp.MustCompile(`-----END [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----`)
	pemEdge  = "---"
)

func Redact(text string) string {
	if text == "" {
		return text
	}
	for _, p := range patterns {
		text = p.replace(text)
	}
	return text
}

func (p pattern) replace(text string) string {
	matches := p.re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	var sb strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if p.group > 0 && m[2*p.group] >= 0 {
			start, end = m[2*p.group], m[2*p.group+1]
		}
		if p.SkipHex && isHex(strings.TrimSpace(text[start:end])) {
			continue
		}
		if p.SkipCall && end < len(text) && text[end] == '(' {
			continue
		}
		if strings.Contains(text[start:end], Mask) {
			continue
		}
		if p.RequireDigit && !strings.ContainsAny(text[start:end], "0123456789") {
			continue
		}
		if p.SkipHex {
			end = start + len(strings.TrimRight(text[start:end], " \t\r\n"))
		}
		sb.WriteString(text[last:start])
		sb.WriteString(maskEdge(text[start:end]))
		last = end
	}
	sb.WriteString(text[last:])
	return sb.String()
}

func maskEdge(value string) string {
	runes := []rune(value)
	if len(runes) <= edgeKeep {
		return Mask
	}
	masked := string(runes[:edgeKeep]) + Mask
	if len(runes) >= edgeMinLen {
		masked += string(runes[len(runes)-edgeKeep:])
	}
	maskedValues.Store(masked, struct{}{})
	return masked
}

var maskedValues sync.Map

func FindMasked(text string) string {
	var found string
	maskedValues.Range(func(key, _ any) bool {
		if strings.Contains(text, key.(string)) {
			found = key.(string)
			return false
		}
		return true
	})
	return found
}

func isHex(s string) bool {
	return s != "" && strings.Trim(s, "0123456789abcdefABCDEF") == ""
}

func Stream() func(string) string {
	inKey := false
	return func(chunk string) string {
		var sb strings.Builder
		for chunk != "" {
			if inKey {
				loc := pemEnd.FindStringIndex(chunk)
				if loc == nil {
					return sb.String()
				}
				sb.WriteString(pemEdge)
				chunk = chunk[loc[1]:]
				inKey = false
				continue
			}
			loc := pemBegin.FindStringIndex(chunk)
			if loc == nil {
				sb.WriteString(Redact(chunk))
				break
			}
			sb.WriteString(Redact(chunk[:loc[0]]))
			sb.WriteString(pemEdge)
			sb.WriteString(Mask)
			maskedValues.Store(pemEdge+Mask, struct{}{})
			chunk = chunk[loc[1]:]
			inKey = true
		}
		return sb.String()
	}
}

type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Unwrap() error { return e.err }

func RedactError(err error) error {
	if err == nil {
		return nil
	}
	msg := Redact(err.Error())
	if msg == err.Error() {
		return err
	}
	return &redactedError{msg: msg, err: err}
}
