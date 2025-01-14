package privval

import (
	"fmt"
	"path/filepath"

	cmtos "github.com/cometbft/cometbft/libs/os"
)

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
