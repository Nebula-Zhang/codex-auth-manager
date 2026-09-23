package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	configFileName = "config.json"
	activeFileName = "auth.json"
)

var (
	errInputClosed  = errors.New("input closed")
	errTargetExists = errors.New("target file already exists")
	namePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
)

type Config struct {
	CodexPath string `json:"codexPath"`
}

type App struct {
	reader     *bufio.Reader
	out        io.Writer
	configPath string
	moveFile   func(string, string) error
}

func NewApp(in io.Reader, out io.Writer, configPath string) *App {
	return &App{
		reader:     bufio.NewReader(in),
		out:        out,
		configPath: configPath,
		moveFile:   moveFileNoReplace,
	}
}

func (a *App) Run() error {
	codexPath, err := loadConfiguredPath(a.configPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintln(a.out, "The saved configuration is invalid or unavailable.")
		}

		codexPath, err = a.promptAndSavePath()
		if err != nil {
			return err
		}
	}

	for {
		files, scanErr := scanAuthFiles(codexPath)
		if scanErr != nil {
			fmt.Fprintln(a.out, "The configured directory cannot be read. Enter it again.")
			codexPath, err = a.promptAndSavePath()
			if err != nil {
				return err
			}
			continue
		}

		a.printMenu(codexPath, files)
		selection, readErr := a.readLine()
		if readErr != nil {
			return readErr
		}

		switch strings.ToUpper(strings.TrimSpace(selection)) {
		case "Q":
			fmt.Fprintln(a.out, "Goodbye.")
			return nil
		case "S":
			if switchErr := a.switchAuth(codexPath, files); switchErr != nil {
				return switchErr
			}
		case "P":
			newPath, pathErr := a.promptAndSavePath()
			if pathErr != nil {
				return pathErr
			}
			codexPath = newPath
			fmt.Fprintln(a.out, "Codex directory updated.")
		case "R":
			if renameErr := a.renameAuth(codexPath, files); renameErr != nil {
				return renameErr
			}
		default:
			fmt.Fprintln(a.out, "Invalid action. Use S, R, P, or Q.")
		}
	}
}

func (a *App) promptAndSavePath() (string, error) {
	for {
		fmt.Fprint(a.out, "Enter the absolute path to your .codex directory: ")
		input, err := a.readLine()
		if err != nil {
			return "", err
		}

		codexPath, validateErr := validateDirectoryPath(input)
		if validateErr != nil {
			fmt.Fprintf(a.out, "Invalid directory: %v\n", validateErr)
			continue
		}

		if saveErr := saveConfig(a.configPath, Config{CodexPath: codexPath}); saveErr != nil {
			return "", fmt.Errorf("failed to save %s beside the program: %w", configFileName, saveErr)
		}

		return codexPath, nil
	}
}

func (a *App) renameAuth(codexPath string, files []string) error {
	if len(files) == 0 {
		fmt.Fprintln(a.out, "No auth files are available to rename.")
		return nil
	}

	selected, selectedOK, err := a.promptForAuthFile(files)
	if err != nil {
		return err
	}
	if !selectedOK {
		fmt.Fprintln(a.out, "Rename cancelled.")
		return nil
	}

	source := filepath.Join(codexPath, selected)
	info, statErr := os.Lstat(source)
	if statErr != nil {
		fmt.Fprintf(a.out, "Cannot access %s.\n", selected)
		return nil
	}
	if !info.Mode().IsRegular() {
		fmt.Fprintf(a.out, "%s is not a regular file.\n", selected)
		return nil
	}

	var targetName string
	var targetPath string
	for {
		fmt.Fprintf(a.out, "Enter a new name for %s, or B to go back: ", selected)
		input, readErr := a.readLine()
		if readErr != nil {
			return readErr
		}
		if strings.EqualFold(strings.TrimSpace(input), "B") {
			fmt.Fprintln(a.out, "Rename cancelled.")
			return nil
		}

		var normalizeErr error
		targetName, normalizeErr = normalizeAuthName(input)
		if normalizeErr != nil {
			fmt.Fprintf(a.out, "Invalid name: %v\n", normalizeErr)
			continue
		}
		if targetName == selected {
			fmt.Fprintln(a.out, "The file already has that name. Choose another name.")
			continue
		}

		targetPath = filepath.Join(codexPath, targetName)
		if _, targetErr := os.Lstat(targetPath); targetErr == nil {
			fmt.Fprintf(a.out, "%s already exists. Choose another name.\n", targetName)
			continue
		} else if !errors.Is(targetErr, fs.ErrNotExist) {
			fmt.Fprintf(a.out, "Cannot check %s. Choose another name.\n", targetName)
			continue
		}
		break
	}

	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Planned change:")
	fmt.Fprintln(a.out)
	fmt.Fprintf(a.out, "%s -> %s\n", selected, targetName)
	fmt.Fprintln(a.out)

	confirmed, confirmErr := a.promptForConfirmation()
	if confirmErr != nil {
		return confirmErr
	}
	if !confirmed {
		fmt.Fprintln(a.out, "Rename cancelled. No files were changed.")
		return nil
	}

	if moveErr := a.moveFile(source, targetPath); moveErr != nil {
		printMoveError(a.out, targetName, moveErr)
		return nil
	}

	fmt.Fprintf(a.out, "Renamed %s to %s.\n", selected, targetName)
	return nil
}

func (a *App) switchAuth(codexPath string, files []string) error {
	if len(files) == 0 {
		fmt.Fprintln(a.out, "No auth files are available to switch.")
		return nil
	}

	selected, selectedOK, err := a.promptForAuthFile(files)
	if err != nil {
		return err
	}
	if !selectedOK {
		fmt.Fprintln(a.out, "Switch cancelled.")
		return nil
	}
	if selected == activeFileName {
		fmt.Fprintln(a.out, "auth.json is already active.")
		return nil
	}

	activePath := filepath.Join(codexPath, activeFileName)
	selectedPath := filepath.Join(codexPath, selected)
	_, activeErr := os.Lstat(activePath)
	activeExists := activeErr == nil
	if activeErr != nil && !errors.Is(activeErr, fs.ErrNotExist) {
		fmt.Fprintln(a.out, "Cannot check the current auth.json file.")
		return nil
	}

	savedName := ""
	savedPath := ""
	if activeExists {
		savedName, savedPath, selectedOK, err = a.promptForSavedName(codexPath, selected)
		if err != nil || !selectedOK {
			return err
		}
	}

	a.printSwitchPlan(selected, savedName, activeExists)
	confirmed, confirmErr := a.promptForConfirmation()
	if confirmErr != nil {
		return confirmErr
	}
	if !confirmed {
		fmt.Fprintln(a.out, "Switch cancelled. No files were changed.")
		return nil
	}

	if activeExists {
		if moveErr := a.moveFile(activePath, savedPath); moveErr != nil {
			printMoveError(a.out, savedName, moveErr)
			return nil
		}

		if moveErr := a.moveFile(selectedPath, activePath); moveErr != nil {
			rollbackErr := a.moveFile(savedPath, activePath)
			if rollbackErr != nil {
				fmt.Fprintf(a.out, "Switch failed and auth.json could not be restored: %v\n", moveErr)
				fmt.Fprintf(a.out, "The previous auth file is still saved as %s.\n", savedName)
				return nil
			}

			fmt.Fprintf(a.out, "Switch failed. The original auth.json was restored: %v\n", moveErr)
			return nil
		}
	} else if moveErr := a.moveFile(selectedPath, activePath); moveErr != nil {
		printMoveError(a.out, activeFileName, moveErr)
		return nil
	}

	fmt.Fprintf(a.out, "Activated %s as auth.json.\n", selected)
	fmt.Fprintln(a.out, "Start a new Codex session to use the active auth file.")
	return nil
}

func (a *App) promptForAuthFile(files []string) (string, bool, error) {
	for {
		fmt.Fprint(a.out, "Enter an auth file number, or B to go back: ")
		input, err := a.readLine()
		if err != nil {
			return "", false, err
		}
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "B") {
			return "", false, nil
		}

		number, parseErr := strconv.Atoi(input)
		if parseErr != nil || number < 1 || number > len(files) {
			fmt.Fprintln(a.out, "Invalid file number.")
			continue
		}

		return files[number-1], true, nil
	}
}

func (a *App) promptForSavedName(codexPath, selectedName string) (string, string, bool, error) {
	for {
		fmt.Fprint(a.out, "Enter a name to save the current auth.json, or B to go back: ")
		input, err := a.readLine()
		if err != nil {
			return "", "", false, err
		}
		if strings.EqualFold(strings.TrimSpace(input), "B") {
			fmt.Fprintln(a.out, "Switch cancelled.")
			return "", "", false, nil
		}

		targetName, normalizeErr := normalizeAuthName(input)
		if normalizeErr != nil {
			fmt.Fprintf(a.out, "Invalid name: %v\n", normalizeErr)
			continue
		}
		if targetName == selectedName {
			fmt.Fprintln(a.out, "That name belongs to the file being activated. Choose another name.")
			continue
		}

		targetPath := filepath.Join(codexPath, targetName)
		if _, statErr := os.Lstat(targetPath); statErr == nil {
			fmt.Fprintf(a.out, "%s already exists. Choose another name.\n", targetName)
			continue
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			fmt.Fprintf(a.out, "Cannot check %s. Choose another name.\n", targetName)
			continue
		}

		return targetName, targetPath, true, nil
	}
}

func (a *App) printSwitchPlan(selectedName, savedName string, activeExists bool) {
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Planned changes:")
	fmt.Fprintln(a.out)
	if activeExists {
		fmt.Fprintf(a.out, "%s -> %s\n", activeFileName, savedName)
	}
	fmt.Fprintf(a.out, "%s -> %s\n", selectedName, activeFileName)
	fmt.Fprintln(a.out)
}

func (a *App) promptForConfirmation() (bool, error) {
	for {
		fmt.Fprint(a.out, "Continue? (Y/N): ")
		input, err := a.readLine()
		if err != nil {
			return false, err
		}

		switch strings.ToUpper(strings.TrimSpace(input)) {
		case "Y", "YES":
			return true, nil
		case "N", "NO", "B":
			return false, nil
		default:
			fmt.Fprintln(a.out, "Enter Y or N.")
		}
	}
}

func (a *App) printMenu(codexPath string, files []string) {
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Codex Auth Manager")
	fmt.Fprintf(a.out, "Codex directory: %s\n", codexPath)
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Auth files:")
	if len(files) == 0 {
		fmt.Fprintln(a.out, "(none)")
	} else {
		for index, name := range files {
			if name == activeFileName {
				fmt.Fprintf(a.out, "%d. %s [ACTIVE]\n", index+1, name)
			} else {
				fmt.Fprintf(a.out, "%d. %s\n", index+1, name)
			}
		}
	}
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "S - Switch auth file")
	fmt.Fprintln(a.out, "R - Rename an auth file")
	fmt.Fprintln(a.out, "P - Change .codex path")
	fmt.Fprintln(a.out, "Q - Quit")
	fmt.Fprint(a.out, "Select action: ")
}

func (a *App) readLine() (string, error) {
	line, err := a.reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		if errors.Is(err, io.EOF) {
			return "", errInputClosed
		}
		return "", err
	}

	return strings.TrimRight(line, "\r\n"), nil
}

func loadConfiguredPath(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return "", err
	}

	return validateDirectoryPath(config.CodexPath)
}

func saveConfig(configPath string, config Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(configPath, data, 0o600)
}

func validateDirectoryPath(input string) (string, error) {
	cleaned := trimPairedQuotes(strings.TrimSpace(input))
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "", errors.New("the path is empty")
	}
	if !filepath.IsAbs(cleaned) {
		return "", errors.New("the path must be absolute")
	}

	cleaned = filepath.Clean(cleaned)
	info, err := os.Stat(cleaned)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", errors.New("the directory does not exist")
		}
		return "", errors.New("the directory cannot be accessed")
	}
	if !info.IsDir() {
		return "", errors.New("the path is not a directory")
	}

	return cleaned, nil
}

func trimPairedQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	first := value[0]
	last := value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1]
	}

	return value
}

func scanAuthFiles(codexPath string) ([]string, error) {
	entries, err := os.ReadDir(codexPath)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "auth") || !strings.HasSuffix(name, ".json") {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, infoErr
		}
		if !info.Mode().IsRegular() {
			continue
		}

		files = append(files, name)
	}

	sort.SliceStable(files, func(i, j int) bool {
		if files[i] == activeFileName {
			return true
		}
		if files[j] == activeFileName {
			return false
		}

		left := strings.ToLower(files[i])
		right := strings.ToLower(files[j])
		if left == right {
			return files[i] < files[j]
		}
		return left < right
	})

	return files, nil
}

func normalizeAuthName(input string) (string, error) {
	name := strings.TrimSpace(input)
	if strings.HasPrefix(name, "auth_") && strings.HasSuffix(name, ".json") {
		name = strings.TrimSuffix(strings.TrimPrefix(name, "auth_"), ".json")
	}

	if !namePattern.MatchString(name) {
		return "", errors.New("use only ASCII letters, numbers, underscores, and hyphens")
	}

	return "auth_" + name + ".json", nil
}

func moveFileNoReplace(source, target string) error {
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("source is unavailable: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return errors.New("source is not a regular file")
	}

	if _, err := os.Lstat(target); err == nil {
		return errTargetExists
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("target cannot be checked: %w", err)
	}

	// Creating a hard link is an atomic no-clobber operation on the local
	// filesystems used by Windows and Linux. Removing the old name completes
	// the move without ever replacing an existing file or reading its content.
	if err := os.Link(source, target); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return errTargetExists
		}
		return fmt.Errorf("target could not be created: %w", err)
	}

	if err := os.Remove(source); err != nil {
		rollbackErr := os.Remove(target)
		if rollbackErr != nil {
			return fmt.Errorf("source could not be removed and rollback failed: %v; rollback: %w", err, rollbackErr)
		}
		return fmt.Errorf("source could not be removed; the operation was rolled back: %w", err)
	}

	return nil
}

func printMoveError(out io.Writer, targetName string, err error) {
	if errors.Is(err, errTargetExists) {
		fmt.Fprintf(out, "%s already exists. No files were changed.\n", targetName)
		return
	}

	fmt.Fprintf(out, "Failed to rename the file: %v\n", err)
}
