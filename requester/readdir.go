package requester

import (
	"os"
)

// ReadDir reads the directory from backend at the given path
func (r *Requester) ReadDir(path string) ([]os.FileInfo, error) {
	d, err := r.makeRequest(workReadDir, path, nil)
	if d != nil {
		return d.([]os.FileInfo), err
	}
	return nil, err
}
