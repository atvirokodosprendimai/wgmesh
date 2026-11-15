package ssh

import "strings"

// Quote returns s as one shell word. A value made only of the characters
// [A-Za-z0-9._:/,@%+=\[\]-] is returned unchanged; anything else, including
// the empty string, is wrapped in single quotes with each embedded single
// quote closed, escaped and reopened (backslash, quote).
func Quote(s string) string {
	if s != "" && isSafeWord(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isSafeWord(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte("._:/,@%+=[]-", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// Cmd builds a shell command line: name is used as given, each argument goes
// through Quote, and the parts are joined by one space.
func Cmd(name string, args ...string) string {
	var b strings.Builder
	b.WriteString(name)
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(Quote(a))
	}
	return b.String()
}
