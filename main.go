package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	configPath, err := configPathBesideExecutable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to locate the program directory: %v\n", err)
		os.Exit(1)
	}

	app := NewApp(os.Stdin, os.Stdout, configPath)
	if err := app.Run(); err != nil {
		if errors.Is(err, errInputClosed) {
			fmt.Fprintln(os.Stderr, "Input was closed.")
		} else {
			fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
		}
		os.Exit(1)
	}
}

func configPathBesideExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}

	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}

	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(executable), configFileName), nil
}
