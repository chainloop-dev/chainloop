package action

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// loadFileOrURL loads a file from a local path or a URL
func loadFileOrURL(fileRef string) ([]byte, error) {
	parts := strings.SplitAfterN(fileRef, "://", 2)
	if len(parts) == 2 {
		scheme := parts[0]
		switch scheme {
		case "http://":
			fallthrough
		case "https://":
			// #nosec G107
			resp, err := http.Get(fileRef)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		default:
			return nil, errors.New("invalid file scheme")
		}
	}

	return os.ReadFile(filepath.Clean(fileRef))
}
