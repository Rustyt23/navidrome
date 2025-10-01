package conf

import (
	"os"
	"path/filepath"

	"github.com/navidrome/navidrome/log"
)

// DiscoveryRoot returns the path to the discovery filesystem root, ensuring it exists.
func DiscoveryRoot() string {
	root := filepath.Join(Server.DataFolder, "discovery")
	if err := os.MkdirAll(root, os.ModePerm); err != nil {
		log.Error("creating discovery root", err)
	}
	return root
}
