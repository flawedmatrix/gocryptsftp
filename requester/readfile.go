package requester

// ReadFile reads the file from backend at the given path
func (r *Requester) ReadFile(path string) ([]byte, error) {
	d, err := r.makeRequest(workReadFile, path, nil)
	if d != nil {
		return d.([]byte), err
	}
	return nil, err
}
