package public

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func scriptPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), name)
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func fakeToolEnv(t *testing.T) (string, []string) {
	t.Helper()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "uname"), `#!/bin/sh
case "$1" in
  -s) printf 'Darwin\n' ;;
  -m) printf 'arm64\n' ;;
  *) printf 'Darwin\n' ;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "curl"), `#!/bin/sh
out=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      out="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
if [ -n "$out" ]; then
  : >"$out"
  exit 0
fi
printf '{"tag_name":"v1.2.3","assets":[{"name":"claudelock-cli-v1.2.3-darwin-arm64.tar.gz"}]}'
`)
	writeExecutable(t, filepath.Join(binDir, "tar"), `#!/bin/sh
dest=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    -C)
      dest="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
path="$dest/claudelock-cli-v1.2.3-darwin-arm64"
printf '#!/bin/sh\nexit 0\n' >"$path"
chmod +x "$path"
`)
	writeExecutable(t, filepath.Join(binDir, "install"), `#!/bin/sh
	last=''
	for arg in "$@"; do
	  last="$arg"
	done
	case "$1" in
	  -d)
	    mkdir -p "$last"
	    exit 0
	    ;;
	esac
	src=''
	dest=''
	while [ "$#" -gt 0 ]; do
	  case "$1" in
	    -m)
	      shift 2
	      ;;
	    -*)
	      shift
	      ;;
	    *)
	      if [ -z "$src" ]; then
	        src="$1"
	      elif [ -z "$dest" ]; then
	        dest="$1"
	      fi
	      shift
	      ;;
	  esac
	done
	cp "$src" "$dest"
	chmod +x "$dest"
	exit 0
	`)
	writeExecutable(t, filepath.Join(binDir, "sudo"), `#!/bin/sh
exec "$@"
`)

	pathEnv := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	return binDir, []string{"PATH=" + pathEnv}
}

func runScript(t *testing.T, path string, env []string, stdin string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", path)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstallAddsManagedAliasForZsh(t *testing.T) {
	_, toolEnv := fakeToolEnv(t)
	homeDir := t.TempDir()
	installPath := filepath.Join(t.TempDir(), "claudelock")
	rcPath := filepath.Join(homeDir, ".zshrc")

	output, err := runScript(t, scriptPath(t, "install.sh"), append(toolEnv, "HOME="+homeDir, "SHELL=/bin/zsh", "INSTALL_PATH="+installPath), "alice\nsecret\n\n")
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	rcData, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	rcContent := string(rcData)
	if !strings.Contains(rcContent, "# claudelock managed start") {
		t.Fatalf("expected managed alias block in zshrc, got:\n%s", rcContent)
	}
	if !strings.Contains(rcContent, `alias claude="claudelock run -- claude"`) {
		t.Fatalf("expected claude alias in zshrc, got:\n%s", rcContent)
	}
	if !strings.Contains(output, "shell_reload:") {
		t.Fatalf("expected shell reload reminder, got:\n%s", output)
	}
}

func TestInstallUpdatesExistingBinaryWithoutRewritingConfig(t *testing.T) {
	_, toolEnv := fakeToolEnv(t)
	homeDir := t.TempDir()
	installDir := t.TempDir()
	installPath := filepath.Join(installDir, "claudelock")
	configPath := filepath.Join(homeDir, ".config", "claudelock.yaml")
	originalBinary := "#!/bin/sh\necho original-binary\n"
	configContent := "server_url: https://example.com\nusername: alice\npassword: secret\n"

	writeExecutable(t, installPath, originalBinary)
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	output, err := runScript(
		t,
		scriptPath(t, "install.sh"),
		append(toolEnv, "HOME="+homeDir, "SHELL=/bin/zsh", "INSTALL_PATH="+installPath),
		"",
	)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	updatedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(updatedConfig) != configContent {
		t.Fatalf("expected config to remain unchanged, got:\n%s", string(updatedConfig))
	}
	backupPaths, err := filepath.Glob(configPath + ".*.bak")
	if err != nil {
		t.Fatalf("glob config backups: %v", err)
	}
	if len(backupPaths) != 0 {
		t.Fatalf("expected update mode to avoid config backups, got: %v", backupPaths)
	}
	updatedBinary, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(updatedBinary) == originalBinary {
		t.Fatalf("expected update mode to replace the existing installed binary")
	}
	if strings.Contains(output, "Username:") {
		t.Fatalf("expected update mode to skip username prompt, got:\n%s", output)
	}
	if strings.Contains(output, "Password:") {
		t.Fatalf("expected update mode to skip password prompt, got:\n%s", output)
	}
	if !strings.Contains(output, "update_status:") {
		t.Fatalf("expected update status in output, got:\n%s", output)
	}
	if strings.Contains(output, "config_path:") {
		t.Fatalf("expected update mode to omit config path, got:\n%s", output)
	}
}

func TestInstallTreatsExistingBinaryWithoutConfigAsFreshInstall(t *testing.T) {
	_, toolEnv := fakeToolEnv(t)
	homeDir := t.TempDir()
	installDir := t.TempDir()
	installPath := filepath.Join(installDir, "claudelock")
	configPath := filepath.Join(homeDir, ".config", "claudelock.yaml")
	originalBinary := "#!/bin/sh\necho original-binary\n"

	writeExecutable(t, installPath, originalBinary)

	output, err := runScript(
		t,
		scriptPath(t, "install.sh"),
		append(toolEnv, "HOME="+homeDir, "SHELL=/bin/zsh", "INSTALL_PATH="+installPath),
		"alice\nsecret\n\n",
	)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	configContent := string(configData)
	if !strings.Contains(configContent, "username: \"alice\"") {
		t.Fatalf("expected config to contain username, got:\n%s", configContent)
	}
	if !strings.Contains(configContent, "password: \"secret\"") {
		t.Fatalf("expected config to contain password, got:\n%s", configContent)
	}
	if !strings.Contains(configContent, "server_url: \"https://claudelock.vps.digisglobal.com\"") {
		t.Fatalf("expected config to contain default server URL, got:\n%s", configContent)
	}
	if strings.Contains(output, "update_status:") {
		t.Fatalf("expected fresh-install output without update status, got:\n%s", output)
	}
	if !strings.Contains(output, "config_path:") {
		t.Fatalf("expected fresh-install output with config path, got:\n%s", output)
	}
}
