package utils

import (
	"fmt"
	"path/filepath"
)

func PGUpgradeDirectory(stateDir string) string {
	return filepath.Join(stateDir, "pg_upgrade")
}

func SegmentPGUpgradeDirectory(stateDir string, contentID int) string {
	return filepath.Join(PGUpgradeDirectory(stateDir), fmt.Sprintf("seg%d", contentID))
}

func MasterPGUpgradeDirectory(stateDir string) string {
	return SegmentPGUpgradeDirectory(stateDir, -1)
}
