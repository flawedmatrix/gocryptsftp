package filetree

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/flawedmatrix/gocryptsftp/gocrypt/configfile"
	"github.com/flawedmatrix/gocryptsftp/gocrypt/contentenc"
	"github.com/flawedmatrix/gocryptsftp/gocrypt/cryptocore"
	"github.com/flawedmatrix/gocryptsftp/gocrypt/nametransform"
	"github.com/flawedmatrix/gocryptsftp/requester"
)

const cacheSize = 16

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 os.FileInfo

// FSAccessor defines an accessor to the real underlying filesystem
type FSAccessor interface {
	ReadFile(path string) ([]byte, error)
	Stat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.FileInfo, error)

	// WriteFile(path string, data []byte) (int64, error)
	// Mkdir(path string) error

	// Rename(path string, target string) error
	// Remove(path string) error
}

// FileTree provides a plaintext view of the filesystem provided by
// fsAccessor
type FileTree struct {
	encryptedRoot string
	masterKey     []byte

	fastCache *FastCache

	fsAccessor FSAccessor
	reqCacher  *requester.Requester

	cCore      *cryptocore.CryptoCore
	cEnc       *contentenc.ContentEnc
	nTransform *nametransform.NameTransform
}

func Init(
	encryptedRoot string,
	password []byte,
	numWorkers int,
	fsAccessor FSAccessor,
) (*FileTree, error) {
	confPath := filepath.Join(encryptedRoot, "gocryptfs.conf")
	confBytes, err := fsAccessor.ReadFile(confPath)
	if err != nil {
		return nil, err
	}
	f, err := ioutil.TempFile("", "gocryptfs.conf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(confBytes); err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	masterKey, conf, err := configfile.LoadAndDecrypt(f.Name(), password)
	if err != nil {
		return nil, err
	}
	cryptoBackend := cryptocore.BackendGoGCM

	hkdf := conf.IsFeatureFlagSet(configfile.FlagHKDF)
	forceDecode := false
	longNames := conf.IsFeatureFlagSet(configfile.FlagLongNames)
	raw64 := conf.IsFeatureFlagSet(configfile.FlagRaw64)
	cCore := cryptocore.New(
		masterKey, cryptoBackend, contentenc.DefaultIVBits,
		hkdf, forceDecode,
	)
	cEnc := contentenc.New(cCore, contentenc.DefaultBS, forceDecode)
	nameTransform := nametransform.New(cCore.EMECipher, longNames, raw64)

	// After the crypto backend is initialized,
	// we can purge the master key from memory.
	for i := range masterKey {
		masterKey[i] = 0
	}

	masterKey = nil

	reqCacher := requester.New(numWorkers, fsAccessor, nameTransform)
	reqCacher.Start()
	return &FileTree{
		encryptedRoot: encryptedRoot,
		masterKey:     masterKey,

		fastCache: NewFastCache(cacheSize),

		cCore:      cCore,
		cEnc:       cEnc,
		nTransform: nameTransform,

		fsAccessor: fsAccessor,
		reqCacher:  reqCacher,
	}, nil
}

func (f *FileTree) ReadFile(plainPath string) ([]byte, error) {
	readFileErr := func(msg string) ([]byte, error) {
		return nil, fmt.Errorf("ReadFile: %s", msg)
	}
	cleanPath := filepath.Clean(plainPath)
	if cleanPath == "/" {
		return readFileErr("/ is a directory")
	}

	plainDirPath := filepath.Dir(cleanPath)
	plainFileName := filepath.Base(cleanPath)

	cipherDirPath, err := f.findPath(plainDirPath)
	if err != nil {
		return readFileErr(fmt.Sprintf("error finding parent path: %s", err))
	}

	item, err := f.findInDir(cipherDirPath, plainFileName)
	if err != nil {
		return readFileErr(err.Error())
	}
	ciphertextPath := filepath.Join(cipherDirPath, item.Name())
	if item.IsDir() {
		return readFileErr(fmt.Sprintf("%s is a directory", ciphertextPath))
	}
	fileBytes, err := f.reqCacher.ReadFile(ciphertextPath)
	if err != nil {
		return readFileErr(fmt.Sprintf("error reading file %s: %s", ciphertextPath, err))
	}

	plainFileBytes, err := f.decryptFile(fileBytes)
	if err != nil {
		return readFileErr(fmt.Sprintf("error decrypting file %s: %s", ciphertextPath, err))
	}
	return plainFileBytes, nil
}

func (f *FileTree) WriteFile(plainPath string, data []byte) (int64, error) {
	return 0, errors.New("Not supported yet")
}

func (f *FileTree) ReadDir(plainPath string) ([]os.FileInfo, error) {
	readDirErr := func(msg string) ([]os.FileInfo, error) {
		return nil, fmt.Errorf("ReadDir: %s", msg)
	}
	cleanPath := filepath.Clean(plainPath)
	ciphertextPath, err := f.findPath(cleanPath)
	if err != nil {
		return readDirErr(fmt.Sprintf("error finding path %s: %s", cleanPath, err))
	}
	var listing []os.FileInfo

	err = f.rangeInDir(ciphertextPath, func(info os.FileInfo, plainName string) bool {
		newInfo := plainFileInfo{
			FileInfo: info,
			name:     plainName,
			size:     int64(f.cEnc.CipherSizeToPlainSize(uint64(info.Size()))),
		}
		listing = append(listing, newInfo)
		return false
	})
	if err != nil {
		return readDirErr(fmt.Sprintf("error listing directory %s: %s", ciphertextPath, err))
	}
	return listing, nil
}

func (f *FileTree) Stat(plainPath string) (os.FileInfo, error) {
	statErr := func(msg string) (os.FileInfo, error) {
		return nil, fmt.Errorf("Stat: %s", msg)
	}
	cleanPath := filepath.Clean(plainPath)

	plainBaseName := filepath.Base(cleanPath)
	cipherPath, err := f.findPath(cleanPath)
	if err != nil {
		return statErr(fmt.Sprintf("error finding path: %s", err))
	}
	item, err := f.fsAccessor.Stat(cipherPath)
	if err != nil {
		return statErr(fmt.Sprintf("error running stat on path: %s", err))
	}
	return plainFileInfo{
		FileInfo: item,
		name:     plainBaseName,
		size:     int64(f.cEnc.CipherSizeToPlainSize(uint64(item.Size()))),
	}, nil
}

func (f *FileTree) Mkdir(plainPath string) error {
	return nil
}

func (f *FileTree) Rename(plainPath string, target string) error {
	return errors.New("Not supported yet")
}

func (f *FileTree) Remove(plainPath string) error {
	return errors.New("Not supported yet")
}

/*
	Helpers ******************************************************************
*/

type listDirFn func(info os.FileInfo, plainName string) (exit bool)

// rangeInDir iterates through all the files/directory located at the real
// encrypted path given by cipherPath. It passes the file info and decrypted to the
// listDirFn, which is the operation to run for each iteration. If the fn
// returns true, then the iteration will exit before the end of the iteration.
func (f *FileTree) rangeInDir(cipherPath string, fn listDirFn) error {
	iv, err := f.reqCacher.ReadFile(filepath.Join(cipherPath, "gocryptfs.diriv"))
	if err != nil {
		return fmt.Errorf("error reading directory IV: %s", err)
	}
	dirListing, err := f.reqCacher.ReadDir(cipherPath)
	if err != nil {
		return fmt.Errorf("error listing directory: %s", err)
	}
	for _, info := range dirListing {
		rName, err := f.reqCacher.DecryptName(info.Name(), iv)
		if err != nil {
			continue
		}
		exit := fn(info, rName)
		if exit {
			break
		}
	}
	return nil
}

// findInDir searches the given directory for an item whose decrypted name
// matches rName. It returns the fileInfo if it exists, and returns an error
// if it doesn't.
func (f *FileTree) findInDir(cipherPath, plainName string) (item os.FileInfo, err error) {
	err = f.rangeInDir(cipherPath, func(info os.FileInfo, decryptedName string) bool {
		if plainName == decryptedName {
			item = info
			return true
		}
		return false
	})
	if err != nil {
		return nil, fmt.Errorf("error iterating %s: %s", cipherPath, err)
	}
	if item == nil {
		return nil, fmt.Errorf("%s not found in %s", plainName, cipherPath)
	}
	return
}

// findPath attempts to find the ciphertext path corresponding to the plaintext
// path by discovering as much as possible about the ciphertext path from the
// fastCache, and then walking the directory tree down the rest of the way. It
// returns the full ciphertext path and an error if the path could not be found.
func (f *FileTree) findPath(plainPath string) (string, error) {
	cleanPath := filepath.Clean(plainPath)
	if cleanPath == "/" {
		return f.encryptedRoot, nil
	}

	dirMapping, found := f.fastCache.Find(cleanPath)
	if found {
		return dirMapping.CiphertextPath, nil
	}

	plainParentPath := filepath.Dir(cleanPath)
	plainDirName := filepath.Base(cleanPath)
	cipherParentPath, err := f.findPath(plainParentPath)
	if err != nil {
		return "", err
	}

	item, err := f.findInDir(cipherParentPath, plainDirName)
	if err != nil {
		return "", err
	}
	ciphertextPath := filepath.Join(cipherParentPath, item.Name())
	if !item.IsDir() {
		return "", fmt.Errorf("%s is not a directory", ciphertextPath)
	}
	f.fastCache.Store(cleanPath, ciphertextPath)
	return ciphertextPath, nil
}
