package watchmod

import (
	"os/user"
	"path/filepath"
)

func expandPath(path string) (string, error) {
	if path[:2] == "~/" {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	}

	return filepath.Abs(path)
}
