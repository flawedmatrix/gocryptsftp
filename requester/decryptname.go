package requester

// DecryptName decrypts the given ciphertext name with the provided
// initialization vector
func (r *Requester) DecryptName(cName string, iv []byte) (string, error) {
	d, err := r.makeRequest(workDecryptName, cName, iv)
	if d != nil {
		return d.(string), err
	}
	return "", err
}
