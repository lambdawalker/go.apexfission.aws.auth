package build

import (
	"archive/zip"

	"io"
	"os"

	"path/filepath"
)

func PackageLambda(binary, archive string) (err error) {
	in, err := os.Open(binary)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(archive), "lambda-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	z := zip.NewWriter(out)
	header := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
	header.SetMode(0755)
	entry, err := z.CreateHeader(header)
	if err == nil {
		_, err = io.Copy(entry, in)
	}
	if e := z.Close(); err == nil {
		err = e
	}
	if e := out.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	// Windows cannot rename over an existing destination.
	if err = os.Remove(archive); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(out.Name(), archive)
}
