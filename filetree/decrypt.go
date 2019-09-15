package filetree

import (
	"io"

	"github.com/flawedmatrix/gocryptsftp/gocrypt/contentenc"
)

// readFileID parses the file ID from the header. It returns an error
// if there is an error parsing or if the file is practically empty.
func (f *FileTree) readFileID(fileBytes []byte) ([]byte, error) {
	if len(fileBytes) <= contentenc.HeaderLen {
		return nil, io.EOF
	}
	buf := fileBytes[:contentenc.HeaderLen]
	h, err := contentenc.ParseHeader(buf)
	if err != nil {
		return nil, err
	}
	return h.ID, nil
}

func (f *FileTree) decryptFile(fileBytes []byte) ([]byte, error) {
	plainLength := f.cEnc.CipherSizeToPlainSize(uint64(len(fileBytes)))
	fileID, err := f.readFileID(fileBytes)
	if err != nil {
		return nil, err
	}
	blocks := f.cEnc.ExplodePlainRange(0, plainLength)
	alignedOffset, _ := blocks[0].JointCiphertextRange(blocks)
	plaintext, err := f.cEnc.DecryptBlocks(fileBytes[alignedOffset:], blocks[0].BlockNo, fileID)
	if err != nil {
		return nil, err
	}
	if cap(plaintext) > (contentenc.MAX_KERNEL_WRITE + contentenc.DefaultBS) {
		return plaintext, nil
	}
	plainBytes := make([]byte, plainLength)
	copy(plainBytes, plaintext)
	f.cEnc.PReqPool.Put(plaintext)
	return plainBytes, nil
}
