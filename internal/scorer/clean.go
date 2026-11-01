package scorer

import (
	"regexp"
	"strings"
)

// A line matching one of these starts the quoted history in a reply or a forward.
var quoteStart = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*-{2,}\s*(original message|forwarded message|ursprungligt meddelande|vidarebefordrat meddelande)\s*-{2,}`),
	regexp.MustCompile(`(?i)^\s*_{10,}\s*$`),
	regexp.MustCompile(`(?i)^\s*(from|fr[åa]n):\s.+`),
	regexp.MustCompile(`(?i)^\s*on\s.+\swrote:\s*$`),
	regexp.MustCompile(`(?i)^\s*den\s.+\sskrev\s*.*:\s*$`),
}

var urlRe = regexp.MustCompile(`(?i)\b(https?://|www\.)\S+`)

// Clean returns only what the sender wrote: no quoted history and no links.
func Clean(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	var kept []string
scan:
	for line := range strings.SplitSeq(body, "\n") {
		// A "From:" on the first lines is part of the mail itself, not a quote
		// header, so only cut once the sender has actually written something.
		if strings.TrimSpace(strings.Join(kept, "")) != "" {
			for _, re := range quoteStart {
				if re.MatchString(line) {
					break scan
				}
			}
		}
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(urlRe.ReplaceAllString(strings.Join(kept, "\n"), ""))
}

// truncate cuts s to at most n runes and reports whether it cut anything.
func truncate(s string, n int) (string, bool) {
	r := []rune(s)
	if len(r) <= n {
		return s, false
	}
	return string(r[:n]), true
}
