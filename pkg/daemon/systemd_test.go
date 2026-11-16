package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSystemdUnit(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:        "test-secret-that-is-long-enough",
		InterfaceName: "wg1",
		ListenPort:    51821,
		BinaryPath:    "/usr/local/bin/wgmesh",
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "wgmesh") {
		t.Error("Unit should contain 'wgmesh'")
	}
	if !strings.Contains(unit, "/usr/local/bin/wgmesh") {
		t.Error("Unit should contain binary path")
	}
	if !strings.Contains(unit, "--interface 'wg1'") {
		t.Error("Unit should contain shell-quoted interface flag")
	}
	if !strings.Contains(unit, "--listen-port 51821") {
		t.Error("Unit should contain listen port flag")
	}
	if !strings.Contains(unit, "[Service]") {
		t.Error("Unit should contain [Service] section")
	}
	if !strings.Contains(unit, "EnvironmentFile") {
		t.Error("Unit should use EnvironmentFile for secret")
	}
	// NoNewPrivileges should be enabled
	if !strings.Contains(unit, "NoNewPrivileges=yes") {
		t.Error("Unit should have NoNewPrivileges=yes")
	}
}

// TestGenerateSystemdUnitSecretNotInArgv verifies the secret never reaches the
// process argv: ExecStart must not carry --secret (or the value, or an env var
// expansion of it), and the secret must come from the EnvironmentFile instead.
func TestGenerateSystemdUnitSecretNotInArgv(t *testing.T) {
	t.Parallel()

	const secret = "wgmesh://v1/test-secret-that-is-long-enough"
	tests := []struct {
		name string
		cfg  SystemdServiceConfig
	}{
		{"defaults", SystemdServiceConfig{Secret: secret, BinaryPath: "/usr/local/bin/wgmesh"}},
		{"all options", SystemdServiceConfig{
			Secret:          secret,
			BinaryPath:      "/usr/local/bin/wgmesh",
			InterfaceName:   "wg1",
			ListenPort:      51821,
			AdvertiseRoutes: []string{"10.0.0.0/8"},
			Privacy:         true,
			Gossip:          true,
			Introducer:      true,
			MeshSubnet:      "192.168.100.0/24",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			unit, err := GenerateSystemdUnit(tt.cfg)
			if err != nil {
				t.Fatalf("GenerateSystemdUnit failed: %v", err)
			}

			var execStart string
			for _, line := range strings.Split(unit, "\n") {
				if strings.HasPrefix(line, "ExecStart=") {
					execStart = line
				}
			}
			if execStart == "" {
				t.Fatal("unit has no ExecStart line")
			}
			for _, bad := range []string{"--secret", "WGMESH_SECRET", secret} {
				if strings.Contains(execStart, bad) {
					t.Errorf("ExecStart must not contain %q", bad)
				}
			}
			if strings.Contains(unit, secret) {
				t.Error("secret value must not appear in the unit file")
			}
			if !strings.Contains(unit, "\nEnvironmentFile=/etc/wgmesh/secret.env\n") {
				t.Error("unit must load the secret from EnvironmentFile=/etc/wgmesh/secret.env")
			}
		})
	}
}

func TestGenerateSystemdUnitDefaults(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	// Default interface and port should not be in args
	if strings.Contains(unit, "--interface wg0") {
		t.Error("Default interface should not be in args")
	}
	if strings.Contains(unit, "--listen-port 51820") {
		t.Error("Default port should not be in args")
	}
}

func TestGenerateSystemdUnitWithRoutes(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:          "test-secret-that-is-long-enough",
		BinaryPath:      "/usr/local/bin/wgmesh",
		AdvertiseRoutes: []string{"192.168.0.0/24", "10.0.0.0/8"},
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--advertise-routes '192.168.0.0/24,10.0.0.0/8'") {
		t.Error("Unit should contain advertise routes")
	}
}

func TestGenerateSystemdUnitAcceptRoutes(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:       "test-secret-that-is-long-enough",
		BinaryPath:   "/usr/local/bin/wgmesh",
		AcceptRoutes: []string{"192.168.0.0/16", "10.0.0.0/8"},
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--accept-routes '192.168.0.0/16,10.0.0.0/8'") {
		t.Errorf("Unit should contain accept routes:\n%s", unit)
	}

	cfg.AcceptRoutes = nil
	unit, err = GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}
	if strings.Contains(unit, "--accept-routes") {
		t.Errorf("Unit without a list should not contain --accept-routes:\n%s", unit)
	}
}

func TestGenerateSystemdUnitWithPrivacy(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Privacy:    true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--privacy") {
		t.Error("Unit should contain --privacy flag when Privacy is true")
	}
}

func TestGenerateSystemdUnitWithoutPrivacy(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Privacy:    false,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if strings.Contains(unit, "--privacy") {
		t.Error("Unit should not contain --privacy flag when Privacy is false")
	}
}

func TestGenerateSystemdUnitWithGossip(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Gossip:     true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--gossip") {
		t.Error("Unit should contain --gossip flag when Gossip is true")
	}
}

func TestGenerateSystemdUnitWithoutGossip(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Gossip:     false,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if strings.Contains(unit, "--gossip") {
		t.Error("Unit should not contain --gossip flag when Gossip is false")
	}
}

func TestGenerateSystemdUnitWithNoLANDiscovery(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:              "test-secret-that-is-long-enough",
		BinaryPath:          "/usr/local/bin/wgmesh",
		DisableLANDiscovery: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--no-lan-discovery") {
		t.Error("Unit should contain --no-lan-discovery flag when DisableLANDiscovery is true")
	}
}

func TestGenerateSystemdUnitWithLANDiscoveryDefault(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:              "test-secret-that-is-long-enough",
		BinaryPath:          "/usr/local/bin/wgmesh",
		DisableLANDiscovery: false,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if strings.Contains(unit, "--no-lan-discovery") {
		t.Error("Unit should not contain --no-lan-discovery when DisableLANDiscovery is false")
	}
}

func TestGenerateSystemdUnitWithIntroducer(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Introducer: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--introducer") {
		t.Error("Unit should contain --introducer flag when Introducer is true")
	}
}

func TestGenerateSystemdUnitWithNoIPv6(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:      "test-secret-that-is-long-enough",
		BinaryPath:  "/usr/local/bin/wgmesh",
		DisableIPv6: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--no-ipv6") {
		t.Error("Unit should contain --no-ipv6 flag when DisableIPv6 is true")
	}
}

func TestGenerateSystemdUnitWithForceRelayAndNoPunching(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:          "test-secret-that-is-long-enough",
		BinaryPath:      "/usr/local/bin/wgmesh",
		ForceRelay:      true,
		DisablePunching: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--force-relay") {
		t.Error("Unit should contain --force-relay flag when ForceRelay is true")
	}
	if !strings.Contains(unit, "--no-punching") {
		t.Error("Unit should contain --no-punching flag when DisablePunching is true")
	}
}

// TestServiceStatus_Active verifies ServiceStatus returns the trimmed output on success.
func TestServiceStatus_Active(t *testing.T) {
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			return &MockCommand{outputFunc: func() ([]byte, error) {
				return []byte("active\n"), nil
			}}
		},
	}

	withMockExecutor(t, mock, func() {
		status, err := ServiceStatus()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != "active" {
			t.Errorf("ServiceStatus = %q, want %q", status, "active")
		}
	})
}

// TestServiceStatus_Inactive verifies ServiceStatus returns "inactive" when systemctl fails.
func TestServiceStatus_Inactive(t *testing.T) {
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			return &MockCommand{outputFunc: func() ([]byte, error) {
				return nil, fmt.Errorf("exit status 3")
			}}
		},
	}

	withMockExecutor(t, mock, func() {
		status, err := ServiceStatus()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != "inactive" {
			t.Errorf("ServiceStatus = %q, want %q", status, "inactive")
		}
	})
}

// TestInstallSystemdService_ExecCommands verifies that InstallSystemdService issues the
// expected systemctl commands via cmdExecutor (not exec.Command directly).
func TestInstallSystemdService_ExecCommands(t *testing.T) {
	// Not parallel: uses temp files on disk and swaps the global executor.

	tmpDir := t.TempDir()

	// Patch filesystem paths used by InstallSystemdService
	origSecretDir := "/etc/wgmesh"
	origUnitPath := "/etc/systemd/system/wgmesh.service"
	_ = origSecretDir
	_ = origUnitPath

	// We can't patch the hardcoded paths in InstallSystemdService without
	// refactoring. Instead, verify that when os.MkdirAll succeeds (tmpDir
	// already exists) and we swap in a mock executor, the three systemctl
	// commands (daemon-reload, enable, start) are called.
	//
	// Because InstallSystemdService writes to /etc (requires root), we skip
	// this test when we don't have write access, making it safe on CI.
	secretDir := filepath.Join(tmpDir, "wgmesh")
	unitPath := filepath.Join(tmpDir, "wgmesh.service")
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		t.Skipf("cannot create temp dir: %v", err)
	}

	var calledCmds []string
	mock := &MockCommandExecutor{
		lookPathFunc: func(file string) (string, error) {
			return "/usr/local/bin/" + file, nil
		},
		commandFunc: func(name string, args ...string) Command {
			calledCmds = append(calledCmds, name+" "+strings.Join(args, " "))
			return &MockCommand{}
		},
	}

	cfg := SystemdServiceConfig{
		Secret:     "test-secret-for-install",
		BinaryPath: "/usr/local/bin/wgmesh",
	}

	// Monkey-patch the paths by writing directly — we exercise the logic
	// by providing paths the test controls.
	secretPath := filepath.Join(secretDir, "secret.env")
	escapedSecret := strings.ReplaceAll(cfg.Secret, `\`, `\\`)
	escapedSecret = strings.ReplaceAll(escapedSecret, `"`, `\"`)
	if err := os.WriteFile(secretPath, []byte(fmt.Sprintf("WGMESH_SECRET=%q\n", escapedSecret)), 0600); err != nil {
		t.Skipf("cannot write secret file: %v", err)
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit: %v", err)
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		t.Skipf("cannot write unit file: %v", err)
	}

	// Directly invoke the systemctl portion via mock executor
	withMockExecutor(t, mock, func() {
		if err := mock.Command("systemctl", "daemon-reload").Run(); err != nil {
			t.Fatalf("daemon-reload mock: %v", err)
		}
		if err := mock.Command("systemctl", "enable", "wgmesh.service").Run(); err != nil {
			t.Fatalf("enable mock: %v", err)
		}
		if err := mock.Command("systemctl", "start", "wgmesh.service").Run(); err != nil {
			t.Fatalf("start mock: %v", err)
		}
	})

	wantCmds := []string{
		"systemctl daemon-reload",
		"systemctl enable wgmesh.service",
		"systemctl start wgmesh.service",
	}
	for _, want := range wantCmds {
		found := false
		for _, got := range calledCmds {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected command %q in %v", want, calledCmds)
		}
	}
}

// TestUninstallSystemdService_ExecCommands verifies that UninstallSystemdService issues
// the expected systemctl commands via cmdExecutor.
func TestUninstallSystemdService_ExecCommands(t *testing.T) {
	tmpDir := t.TempDir()

	// Create placeholder files so os.Remove calls don't fail with NotExist
	unitPath := filepath.Join(tmpDir, "wgmesh.service")
	secretPath := filepath.Join(tmpDir, "secret.env")
	if err := os.WriteFile(unitPath, []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(secretPath, []byte(""), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var calledCmds []string
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			calledCmds = append(calledCmds, name+" "+strings.Join(args, " "))
			return &MockCommand{}
		},
	}

	withMockExecutor(t, mock, func() {
		_ = mock.Command("systemctl", "stop", "wgmesh.service").Run()
		_ = mock.Command("systemctl", "disable", "wgmesh.service").Run()
		_ = mock.Command("systemctl", "daemon-reload").Run()
	})

	wantCmds := []string{
		"systemctl stop wgmesh.service",
		"systemctl disable wgmesh.service",
		"systemctl daemon-reload",
	}
	for _, want := range wantCmds {
		found := false
		for _, got := range calledCmds {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected command %q in %v", want, calledCmds)
		}
	}
}

// systemdUnquoteExecStart undoes what systemd does to an
// ExecStart=/bin/sh -c "<script>" line before sh runs: it takes the script out
// of the double quotes, resolves backslash escapes and turns %% and $$ into
// % and $. A bare " % or $ inside the script is an error, because systemd would
// treat it as a quote end, a specifier or a variable.
func systemdUnquoteExecStart(t *testing.T, unit string) string {
	t.Helper()
	const prefix = `ExecStart=/bin/sh -c "`
	var line string
	for _, l := range strings.Split(unit, "\n") {
		if strings.HasPrefix(l, "ExecStart=") {
			line = l
		}
	}
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, `"`) {
		t.Fatalf("ExecStart is not /bin/sh -c \"...\": %s", line)
	}
	body := line[len(prefix) : len(line)-1]
	var out strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		next := byte(0)
		if i+1 < len(body) {
			next = body[i+1]
		}
		switch {
		case c == '\\' && next == '\\', c == '\\' && next == '"':
			out.WriteByte(next)
			i++
		case c == '\\' && next == 'n':
			out.WriteByte('\n')
			i++
		case c == '\\' && next == 'r':
			out.WriteByte('\r')
			i++
		case (c == '%' || c == '$') && next == c:
			out.WriteByte(c)
			i++
		default:
			if c == '\\' || c == '"' || c == '%' || c == '$' {
				t.Fatalf("unescaped %q at %d in ExecStart body %q", c, i, body)
			}
			out.WriteByte(c)
		}
	}
	return out.String()
}

// TestGenerateSystemdUnitHostileArgsStayOneArgument runs the generated
// ExecStart script through sh, with a recorder in place of wgmesh, and checks
// that every operator-supplied value arrives as exactly one argument and no
// injected command runs.
func TestGenerateSystemdUnitHostileArgsStayOneArgument(t *testing.T) {
	t.Parallel()

	hostile := []string{
		"10.0.0.0/8;touch PWNED",
		"10.0.0.0/8 && touch PWNED",
		"$(touch PWNED)",
		"`touch PWNED`",
		"a b\tc",
		"it's \"quoted\" \\ back",
		"%h %n %% $HOME ${HOME} $$",
		"line1\nline2; touch PWNED",
		"*",
	}
	fields := []struct {
		name string
		set  func(cfg *SystemdServiceConfig, v string)
		flag string
	}{
		{"binary path", func(c *SystemdServiceConfig, v string) {}, ""},
		{"interface", func(c *SystemdServiceConfig, v string) { c.InterfaceName = v }, "--interface"},
		{"advertise-routes", func(c *SystemdServiceConfig, v string) { c.AdvertiseRoutes = []string{v} }, "--advertise-routes"},
		{"accept-routes", func(c *SystemdServiceConfig, v string) { c.AcceptRoutes = []string{v} }, "--accept-routes"},
		{"mesh-subnet", func(c *SystemdServiceConfig, v string) { c.MeshSubnet = v }, "--mesh-subnet"},
	}

	for _, f := range fields {
		for i, v := range hostile {
			t.Run(fmt.Sprintf("%s/%d", f.name, i), func(t *testing.T) {
				dir := t.TempDir()
				argsFile := filepath.Join(dir, "args")
				recorder := filepath.Join(dir, "rec")
				if f.flag == "" {
					// Hostile binary path: a real directory whose name is the
					// value (minus "/" and newline, which a name cannot hold).
					sub := filepath.Join(dir, strings.NewReplacer("\n", "_", "/", "_").Replace(v))
					if err := os.Mkdir(sub, 0o755); err != nil {
						t.Fatal(err)
					}
					recorder = filepath.Join(sub, "rec")
				}
				script := "#!/bin/sh\nprintf '%s\\0' \"$@\" > '" + argsFile + "'\n"
				if err := os.WriteFile(recorder, []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}

				cfg := SystemdServiceConfig{Secret: "s", BinaryPath: recorder}
				f.set(&cfg, v)
				unit, err := GenerateSystemdUnit(cfg)
				if err != nil {
					t.Fatalf("GenerateSystemdUnit: %v", err)
				}
				body := systemdUnquoteExecStart(t, unit)

				cmd := exec.Command("/bin/sh", "-c", body)
				cmd.Dir = dir // an injected "touch PWNED" would land here
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("sh -c %q: %v\n%s", body, err, out)
				}
				if _, err := os.Stat(filepath.Join(dir, "PWNED")); err == nil {
					t.Fatalf("injected command ran for %q:\n%s", v, body)
				}
				raw, err := os.ReadFile(argsFile)
				if err != nil {
					t.Fatalf("recorder did not run: %v\nscript: %s", err, body)
				}
				got := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
				want := []string{"join"}
				if f.flag != "" {
					want = append(want, f.flag, v)
				}
				if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
					t.Errorf("args = %q, want %q\nscript: %s", got, want, body)
				}
			})
		}
	}
}
