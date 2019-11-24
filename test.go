package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func main() {
	b, err := ioutil.ReadFile("foo.tar")
	if err != nil {
		panic(err)
	}
	r := bytes.NewReader(b)
	filePaths := make([]string, 0)
	logrus.Infof("sha %x", sha256.Sum256(b))
	oldHeaders := make([]*tar.Header, 0)
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		oldHeaders = append(oldHeaders, hdr)
		root := "/Users/colewippern/go/src/github.com/cvgw/kaniko"

		filePaths, err = extractFile(root, hdr, tr, filePaths)
		if err != nil {
			panic(err)
		}
	}
	logrus.Info(filePaths)
	oldHeaderVals := make([]tar.Header, 0)
	for _, hr := range oldHeaders {
		oldHeaderVals = append(oldHeaderVals, *hr)
	}
	logrus.Info(oldHeaderVals)

	buf := bytes.NewBuffer(make([]byte, 0))

	newHeaders := make([]*tar.Header, 0)
	tw := tar.NewWriter(buf)
	defer tw.Close()
	src := "/Users/colewippern/go/src/github.com/cvgw/kaniko"
	filePaths = filesWithParentDirs(filePaths)
	for _, filepath := range filePaths {
		fi, err := os.Stat(filepath)
		if err != nil {
			panic(err)
		}
		//if !fi.Mode().IsRegular() {
		//  logrus.Info("fi is no reg?")
		//  continue
		//}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			panic(err)
		}

		header.Name = strings.TrimPrefix(strings.Replace(filepath, src, "", -1), "/")

		newHeaders = append(newHeaders, header)
		if err := tw.WriteHeader(header); err != nil {
			panic(err)
		}
		if !fi.Mode().IsRegular() {
			continue
		}

		f, err := os.Open(filepath)
		if err != nil {
			panic(err)
		}

		if _, err := io.Copy(tw, f); err != nil {
			panic(err)
		}

		f.Close()
	}
	newHeaderVals := make([]tar.Header, 0)
	for _, hr := range newHeaders {
		newHeaderVals = append(newHeaderVals, *hr)
	}
	logrus.Info(newHeaderVals)
	logrus.Infof("sha %x", sha256.Sum256(buf.Bytes()))

}

// ParentDirectories returns a list of paths to all parent directories
// Ex. /some/temp/dir -> [/, /some, /some/temp, /some/temp/dir]
func ParentDirectories(path string) []string {
	path = filepath.Clean(path)
	dirPath := "/Users/colewippern/go/src/github.com/cvgw/kaniko"
	path = strings.TrimPrefix(strings.Replace(path, dirPath, "", -1), "/")
	paths := make([]string, 0)
	dirs := strings.Split(path, "/")
	dir := ""
	for _, d := range dirs[:len(dirs)-1] {
		p := filepath.Join(dirPath, dir, d)
		logrus.Infof("dir %v", p)
		paths = append(paths, p)
		dir = filepath.Join(dir, d)
	}
	return paths
}

func filesWithParentDirs(files []string) []string {
	filesSet := map[string]bool{}

	for _, file := range files {
		file = filepath.Clean(file)
		filesSet[file] = true

		for _, dir := range ParentDirectories(file) {
			dir = filepath.Clean(dir)
			filesSet[dir] = true
		}
	}

	newFiles := []string{}
	for file := range filesSet {
		newFiles = append(newFiles, file)
	}

	return newFiles
}

func extractFile(dest string, hdr *tar.Header, tr io.Reader, filePaths []string) ([]string, error) {
	path := filepath.Join(dest, filepath.Clean(hdr.Name))
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	mode := hdr.FileInfo().Mode()
	uid := hdr.Uid
	gid := hdr.Gid

	switch hdr.Typeflag {
	case tar.TypeReg:
		logrus.Debugf("creating file %s", path)
		// It's possible a file is in the tar before its directory,
		// or a file was copied over a directory prior to now
		fi, err := os.Stat(dir)
		if os.IsNotExist(err) || !fi.IsDir() {
			logrus.Debugf("base %s for file %s does not exist. Creating.", base, path)

			if err := os.MkdirAll(dir, 0755); err != nil {
				return filePaths, err
			}
		}
		// Check if something already exists at path (symlinks etc.)

		filePaths = append(filePaths, path)
		currFile, err := os.Create(path)
		if err != nil {
			return filePaths, err
		}
		if _, err = io.Copy(currFile, tr); err != nil {
			return filePaths, err
		}
		if err = setFilePermissions(path, mode, uid, gid); err != nil {
			return filePaths, err
		}
		if err := os.Chtimes(path, hdr.AccessTime, hdr.ModTime); err != nil {
			panic(err)
		}
		currFile.Close()
	case tar.TypeDir:
		logrus.Tracef("creating dir %s", path)
		if err := mkdirAllWithPermissions(path, mode, uid, gid); err != nil {
			return filePaths, err
		}

	case tar.TypeLink:
		logrus.Tracef("link from %s to %s", hdr.Linkname, path)

		// The base directory for a link may not exist before it is created.
		if err := os.MkdirAll(dir, 0755); err != nil {
			return filePaths, err
		}

		link := filepath.Clean(filepath.Join(dest, hdr.Linkname))
		if err := os.Link(link, path); err != nil {
			return filePaths, err
		}

	case tar.TypeSymlink:
		logrus.Tracef("symlink from %s to %s", hdr.Linkname, path)
		// The base directory for a symlink may not exist before it is created.
		if err := os.MkdirAll(dir, 0755); err != nil {
			return filePaths, err
		}

		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return filePaths, err
		}
	}
	return filePaths, nil
}

// CreateFile creates a file at path and copies over contents from the reader
func CreateFile(path string, reader io.Reader, perm os.FileMode, uid uint32, gid uint32) error {
	// Create directory path if it doesn't exist
	baseDir := filepath.Dir(path)
	if info, err := os.Lstat(baseDir); os.IsNotExist(err) {
		logrus.Tracef("baseDir %s for file %s does not exist. Creating.", baseDir, path)
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return err
		}
	} else {
		switch mode := info.Mode(); {
		case mode&os.ModeSymlink != 0:
			logrus.Infof("destination cannot be a symlink %v", baseDir)
			return errors.New("destination cannot be a symlink")
		}
	}
	dest, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dest.Close()
	if _, err := io.Copy(dest, reader); err != nil {
		return err
	}
	return setFilePermissions(path, perm, int(uid), int(gid))
}

func mkdirAllWithPermissions(path string, mode os.FileMode, uid, gid int) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	// In some cases, MkdirAll doesn't change the permissions, so run Chmod
	// Must chmod after chown because chown resets the file mode.
	return os.Chmod(path, mode)
}

func setFilePermissions(path string, mode os.FileMode, uid, gid int) error {
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	// manually set permissions on file, since the default umask (022) will interfere
	// Must chmod after chown because chown resets the file mode.
	return os.Chmod(path, mode)
}
