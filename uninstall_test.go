package public

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallRemovesManagedAliasBlock(t *testing.T) {
	homeDir := t.TempDir()
	installPath := filepath.Join(t.TempDir(), "claudelock")
	if err := os.WriteFile(installPath, []byte("binary"), 0755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	rcPath := filepath.Join(homeDir, ".zshrc")
	content := strings.Join([]string{
		"alias ll='ls -l'",
		"# claudelock managed start",
		`alias claude="claudelock run -- claude"`,
		"# claudelock managed end",
		"alias gs='git status'",
	}, "\n") + "\n"
	if err := os.WriteFile(rcPath, []byte(content), 0644); err != nil {
		t.Fatalf("write zshrc: %v", err)
	}

	output, err := runScript(t, scriptPath(t, "uninstall.sh"), []string{"HOME=" + homeDir, "INSTALL_PATH=" + installPath}, "")
	if err != nil {
		t.Fatalf("uninstall.sh failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(installPath); !os.IsNotExist(err) {
		t.Fatalf("expected install path removed, stat err=%v", err)
	}

	rcData, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	rcContent := string(rcData)
	if strings.Contains(rcContent, "# claudelock managed start") || strings.Contains(rcContent, `alias claude="claudelock run -- claude"`) {
		t.Fatalf("expected managed alias block removed, got:\n%s", rcContent)
	}
	if !strings.Contains(rcContent, "alias ll='ls -l'") || !strings.Contains(rcContent, "alias gs='git status'") {
		t.Fatalf("expected unrelated aliases preserved, got:\n%s", rcContent)
	}
}
