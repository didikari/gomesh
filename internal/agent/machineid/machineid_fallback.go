//go:build !windows && !linux && !darwin

package machineid

import "errors"

func getPlatformID() (string, error) {
	return "", errors.New("platform not supported")
}
