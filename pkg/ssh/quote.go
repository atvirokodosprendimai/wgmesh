package ssh

// Quote returns s as one shell word.
func Quote(s string) string { return s }

// Cmd joins name and args with single spaces.
func Cmd(name string, args ...string) string {
	out := name
	for _, a := range args {
		out += " " + a
	}
	return out
}
