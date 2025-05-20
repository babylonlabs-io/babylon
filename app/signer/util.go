package signer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bgentry/speakeasy"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/mattn/go-isatty"
)

// EnsureDirs ensures the directories of the given paths exist.
func EnsureDirs(paths ...string) error {
	// Check file path of bls key
	for _, path := range paths {
		if path == "" {
			return fmt.Errorf("filePath for bls key not set")
		}
		if err := cmtos.EnsureDir(filepath.Dir(path), 0777); err != nil {
			return fmt.Errorf("failed to ensure key path dir: %w", err)
		}
	}
	return nil
}

// NewBlsPassword returns a password from the user prompt.
// It asks for the password twice to confirm and uses a secure input method.
// Users have up to 3 attempts to enter matching passwords.
func NewBlsPassword() string {
	inBuf := bufio.NewReader(os.Stdin)

	const maxAttempts = 3
	var attempt int

	for attempt = 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			fmt.Printf("\nPasswords didn't match. Attempt %d of %d\n", attempt+1, maxAttempts)
		}

		password, err := GetBlsPasswordInput("Enter your BLS password (input will be hidden): ", inBuf)
		if err != nil {
			cmtos.Exit(fmt.Sprintf("failed to get BLS password: %v", err.Error()))
		}

		confirmPassword, err := GetBlsPasswordInput("Confirm your BLS password (input will be hidden): ", inBuf)
		if err != nil {
			cmtos.Exit(fmt.Sprintf("failed to get BLS password confirmation: %v", err.Error()))
		}

		if password == confirmPassword {
			return password
		}

		fmt.Println("ERROR: Passwords do not match")
	}

	cmtos.Exit("Failed to get matching passwords after 3 attempts")
	return ""
}

// GetBlsPasswordInput is a custom password input function that doesn't enforce
// any password length restrictions but still hides the input.
func GetBlsPasswordInput(prompt string, buf *bufio.Reader) (string, error) {
	if inputIsTty() {
		password, err := speakeasy.FAsk(os.Stderr, prompt)
		if err != nil {
			return "", err
		}
		return password, nil
	}

	// If not a TTY, read from buffer
	password, err := readLineFromBuf(buf)
	if err != nil {
		return "", err
	}

	return password, nil
}

// Helper function to determine if input is from a terminal
func inputIsTty() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
}

// Helper function to read a line from a buffer
func readLineFromBuf(buf *bufio.Reader) (string, error) {
	pass, err := buf.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(pass), nil
}

// GetBlsUnlockPasswordFromPrompt prompts the user for a password once, without confirmation.
// This is suitable for unlock operations where we only need to check if the password
// is correct for an existing key.
func GetBlsUnlockPasswordFromPrompt() string {
	inBuf := bufio.NewReader(os.Stdin)

	password, err := GetBlsPasswordInput("Enter your BLS key password (input will be hidden): ", inBuf)
	if err != nil {
		cmtos.Exit(fmt.Sprintf("failed to get BLS password: %v", err.Error()))
	}

	return password
}
