package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestNormalizeAuthName(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "work", want: "auth_work.json", ok: true},
		{input: " auth_work.json ", want: "auth_work.json", ok: true},
		{input: "personal_2", want: "auth_personal_2.json", ok: true},
		{input: "company-us", want: "auth_company-us.json", ok: true},
		{input: "", ok: false},
		{input: "auth.json", ok: false},
		{input: "auth_.json", ok: false},
		{input: "work.json", ok: false},
		{input: "../work", ok: false},
		{input: "work name", ok: false},
		{input: "company:us", ok: false},
		{input: "personal account", ok: false},
		{input: "work-account-01", want: "auth_work-account-01.json", ok: true},
		{input: "\u5de5\u4f5c", ok: false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeAuthName(test.input)
			if test.ok && err != nil {
				t.Fatalf("normalizeAuthName() error = %v", err)
			}
			if !test.ok && err == nil {
				t.Fatalf("normalizeAuthName() unexpectedly succeeded with %q", got)
			}
			if got != test.want {
				t.Fatalf("normalizeAuthName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidateDirectoryPath(t *testing.T) {
	directory := t.TempDir()

	got, err := validateDirectoryPath("  \"" + directory + "\"  ")
	if err != nil {
		t.Fatalf("validateDirectoryPath() error = %v", err)
	}
	if got != filepath.Clean(directory) {
		t.Fatalf("validateDirectoryPath() = %q, want %q", got, filepath.Clean(directory))
	}

	if _, err := validateDirectoryPath("relative/path"); err == nil {
		t.Fatal("validateDirectoryPath() accepted a relative path")
	}
	if _, err := validateDirectoryPath(filepath.Join(directory, "missing")); err == nil {
		t.Fatal("validateDirectoryPath() accepted a missing path")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(t.TempDir(), configFileName)

	if err := saveConfig(configPath, Config{CodexPath: directory}); err != nil {
		t.Fatalf("saveConfig() error = %v", err)
	}

	got, err := loadConfiguredPath(configPath)
	if err != nil {
		t.Fatalf("loadConfiguredPath() error = %v", err)
	}
	if got != filepath.Clean(directory) {
		t.Fatalf("loadConfiguredPath() = %q, want %q", got, filepath.Clean(directory))
	}
}

func TestScanAuthFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{
		"auth.json",
		"auth_work.json",
		"auth-old.json",
		"auth2.json",
		"other_auth.json",
		"auth.json.backup",
		"Auth_upper.json",
	} {
		writeTestFile(t, filepath.Join(directory, name), name)
	}
	if err := os.Mkdir(filepath.Join(directory, "auth_directory.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := scanAuthFiles(directory)
	if err != nil {
		t.Fatalf("scanAuthFiles() error = %v", err)
	}
	want := []string{"auth.json", "auth-old.json", "auth2.json", "auth_work.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scanAuthFiles() = %#v, want %#v", got, want)
	}
}

func TestMoveFileNoReplace(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "auth_work.json")
	target := filepath.Join(directory, activeFileName)
	writeTestFile(t, source, "secret")

	if err := moveFileNoReplace(source, target); err != nil {
		t.Fatalf("moveFileNoReplace() error = %v", err)
	}
	if _, err := os.Stat(source); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("source still exists or returned wrong error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "secret" {
		t.Fatalf("target content = %q, want %q", data, "secret")
	}
}

func TestMoveFileNoReplaceRefusesConflict(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "auth_work.json")
	target := filepath.Join(directory, activeFileName)
	writeTestFile(t, source, "source")
	writeTestFile(t, target, "target")

	err := moveFileNoReplace(source, target)
	if !errors.Is(err, errTargetExists) {
		t.Fatalf("moveFileNoReplace() error = %v, want errTargetExists", err)
	}
	assertFileContent(t, source, "source")
	assertFileContent(t, target, "target")
}

func TestAppSwitchesAndSavesCurrentAuth(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")
	writeTestFile(t, filepath.Join(directory, "auth_work.json"), "work")

	output := runConfiguredApp(t, directory, "S\n2\nold\nY\nQ\n", nil)
	if !strings.Contains(output, "auth.json -> auth_old.json") {
		t.Fatalf("output did not contain the saved-name plan:\n%s", output)
	}
	if !strings.Contains(output, "Activated auth_work.json as auth.json.") {
		t.Fatalf("output did not contain the activation message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "work")
	assertFileContent(t, filepath.Join(directory, "auth_old.json"), "active")
}

func TestAppActivatesWhenNoCurrentAuthExists(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "auth_work.json"), "work")

	output := runConfiguredApp(t, directory, "S\n1\nY\nQ\n", nil)
	if !strings.Contains(output, "Activated auth_work.json as auth.json.") {
		t.Fatalf("output did not contain the activation message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "work")
	if _, err := os.Stat(filepath.Join(directory, "auth_work.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old file still exists or returned wrong error: %v", err)
	}
}

func TestAppSwitchCancellationChangesNothing(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")
	writeTestFile(t, filepath.Join(directory, "auth_work.json"), "work")

	output := runConfiguredApp(t, directory, "S\n2\nold\nN\nQ\n", nil)
	if !strings.Contains(output, "Switch cancelled. No files were changed.") {
		t.Fatalf("output did not contain the cancellation message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "active")
	assertFileContent(t, filepath.Join(directory, "auth_work.json"), "work")
	if _, err := os.Stat(filepath.Join(directory, "auth_old.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("saved file exists after cancellation or returned wrong error: %v", err)
	}
}

func TestAppSwitchRollbackRestoresCurrentAuth(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")
	writeTestFile(t, filepath.Join(directory, "auth_work.json"), "work")

	moveCalls := 0
	move := func(source, target string) error {
		moveCalls++
		if moveCalls == 2 {
			return fs.ErrPermission
		}
		return moveFileNoReplace(source, target)
	}
	output := runConfiguredApp(t, directory, "S\n2\nold\nY\nQ\n", move)
	if !strings.Contains(output, "Switch failed. The original auth.json was restored") {
		t.Fatalf("output did not contain the rollback message:\n%s", output)
	}
	if moveCalls != 3 {
		t.Fatalf("move call count = %d, want 3", moveCalls)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "active")
	assertFileContent(t, filepath.Join(directory, "auth_work.json"), "work")
	if _, err := os.Stat(filepath.Join(directory, "auth_old.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("saved file exists after rollback or returned wrong error: %v", err)
	}
}

func TestAppSwitchRejectsSelectedFileAsSavedName(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")
	writeTestFile(t, filepath.Join(directory, "auth_work.json"), "work")

	output := runConfiguredApp(t, directory, "S\n2\nwork\nold\nY\nQ\n", nil)
	if !strings.Contains(output, "That name belongs to the file being activated.") {
		t.Fatalf("output did not contain the conflict message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "work")
	assertFileContent(t, filepath.Join(directory, "auth_old.json"), "active")
}

func TestAppRequiresActionBeforeFileNumber(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "auth_work.json"), "work")

	output := runConfiguredApp(t, directory, "1\nQ\n", nil)
	if !strings.Contains(output, "Invalid action. Use S, R, P, or Q.") {
		t.Fatalf("output did not require an action first:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, "auth_work.json"), "work")
}

func TestAppRenamesCurrentAuth(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")

	output := runConfiguredApp(t, directory, "R\n1\nauth_work.json\nY\nQ\n", nil)
	if !strings.Contains(output, "Renamed auth.json to auth_work.json.") {
		t.Fatalf("output did not contain the rename message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, "auth_work.json"), "active")
}

func TestAppRenamesNonActiveAuthFile(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")
	writeTestFile(t, filepath.Join(directory, "auth_old.json"), "old")

	output := runConfiguredApp(t, directory, "R\n2\npersonal\nY\nQ\n", nil)
	if !strings.Contains(output, "auth_old.json -> auth_personal.json") {
		t.Fatalf("output did not contain the rename plan:\n%s", output)
	}
	if !strings.Contains(output, "Renamed auth_old.json to auth_personal.json.") {
		t.Fatalf("output did not contain the rename message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "active")
	assertFileContent(t, filepath.Join(directory, "auth_personal.json"), "old")
	if _, err := os.Stat(filepath.Join(directory, "auth_old.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old file still exists or returned wrong error: %v", err)
	}
}

func TestAppRenameCancellationChangesNothing(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")
	writeTestFile(t, filepath.Join(directory, "auth_old.json"), "old")

	output := runConfiguredApp(t, directory, "R\n2\npersonal\nN\nQ\n", nil)
	if !strings.Contains(output, "Rename cancelled. No files were changed.") {
		t.Fatalf("output did not contain the cancellation message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "active")
	assertFileContent(t, filepath.Join(directory, "auth_old.json"), "old")
}

func TestAppRenameRefusesExistingTargetAndPromptsAgain(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")
	writeTestFile(t, filepath.Join(directory, "auth_old.json"), "old")
	writeTestFile(t, filepath.Join(directory, "auth_taken.json"), "taken")

	output := runConfiguredApp(t, directory, "R\n2\ntaken\npersonal\nY\nQ\n", nil)
	if !strings.Contains(output, "auth_taken.json already exists. Choose another name.") {
		t.Fatalf("output did not contain the conflict message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "active")
	assertFileContent(t, filepath.Join(directory, "auth_taken.json"), "taken")
	assertFileContent(t, filepath.Join(directory, "auth_personal.json"), "old")
}

func TestAppRenameFailureLeavesSource(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, activeFileName), "active")

	move := func(string, string) error { return fs.ErrPermission }
	output := runConfiguredApp(t, directory, "R\n1\nwork\nY\nQ\n", move)
	if !strings.Contains(output, "Failed to rename the file") {
		t.Fatalf("output did not contain the failure message:\n%s", output)
	}
	assertFileContent(t, filepath.Join(directory, activeFileName), "active")
	if _, err := os.Stat(filepath.Join(directory, "auth_work.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("target exists after failed move or returned wrong error: %v", err)
	}
}

func TestAppFirstRunAcceptsQuotedAbsolutePath(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(t.TempDir(), configFileName)
	input := "\"" + directory + "\"\nQ\n"
	var output bytes.Buffer
	app := NewApp(strings.NewReader(input), &output, configPath)

	if err := app.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	configuredPath, err := loadConfiguredPath(configPath)
	if err != nil {
		t.Fatalf("loadConfiguredPath() error = %v", err)
	}
	if configuredPath != filepath.Clean(directory) {
		t.Fatalf("configured path = %q, want %q", configuredPath, filepath.Clean(directory))
	}
}

func TestAppRecoversFromInvalidConfigAndRelativeInput(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(t.TempDir(), configFileName)
	writeTestFile(t, configPath, "not-json")
	input := "relative/path\n" + directory + "\nQ\n"
	var output bytes.Buffer
	app := NewApp(strings.NewReader(input), &output, configPath)

	if err := app.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "The saved configuration is invalid or unavailable.") {
		t.Fatalf("output did not identify the invalid config:\n%s", text)
	}
	if !strings.Contains(text, "the path must be absolute") {
		t.Fatalf("output did not reject the relative path:\n%s", text)
	}
}

func TestAppRecoversFromMissingConfiguredDirectory(t *testing.T) {
	missingDirectory := filepath.Join(t.TempDir(), "missing")
	replacementDirectory := t.TempDir()
	configPath := filepath.Join(t.TempDir(), configFileName)
	data := []byte("{\"codexPath\":" + strconv.Quote(missingDirectory) + "}\n")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	app := NewApp(strings.NewReader(replacementDirectory+"\nQ\n"), &output, configPath)
	if err := app.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), "The saved configuration is invalid or unavailable.") {
		t.Fatalf("output did not identify the missing configured directory:\n%s", output.String())
	}

	configuredPath, err := loadConfiguredPath(configPath)
	if err != nil {
		t.Fatalf("loadConfiguredPath() error = %v", err)
	}
	if configuredPath != filepath.Clean(replacementDirectory) {
		t.Fatalf("configured path = %q, want %q", configuredPath, filepath.Clean(replacementDirectory))
	}
}

func runConfiguredApp(t *testing.T, directory, input string, move func(string, string) error) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), configFileName)
	if err := saveConfig(configPath, Config{CodexPath: directory}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	app := NewApp(strings.NewReader(input), &output, configPath)
	if move != nil {
		app.moveFile = move
	}
	if err := app.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return output.String()
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s content = %q, want %q", path, data, want)
	}
}
