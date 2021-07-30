// Copyright © 2020 Jose Riguera <jriguera@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package tar

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "kubefoundry/internal/log"
	glob "kubefoundry/pkg/glob"
)

// Executor
type Tar struct {
	BasePath     string
	dstPath      string
	srcPath      string
	SkipDirGlob  *glob.Glob
	SkipFileGlob *glob.Glob
	FileGlob     *glob.Glob
	DirGlob      *glob.Glob
	file         *os.File
	ctx          context.Context
	mw           io.Writer
	tw           *tar.Writer
	mu           sync.Mutex
	log          log.Logger
}

func NewTar(basepath string, l log.Logger, outputs ...io.Writer) *Tar {
	var mw io.Writer = nil
	fglob, _ := glob.New("*")
	dglob, _ := glob.New("*")
	if len(outputs) > 0 {
		mw = io.MultiWriter(outputs...)
	}
	t := Tar{
		BasePath:     basepath,
		SkipDirGlob:  nil,
		SkipFileGlob: nil,
		FileGlob:     fglob,
		DirGlob:      dglob,
		file:         nil,
		mw:           mw,
		tw:           tar.NewWriter(mw),
		log:          l,
	}
	return &t
}

func NewTarFile(tarfile string, l log.Logger) (*Tar, error) {
	target, err := os.Create(tarfile)
	if err != nil {
		return nil, err
	}
	t := NewTar(".", l, target)
	t.file = target
	return t, err
}

func (t *Tar) Close() {
	t.tw.Close()
	if t.file != nil {
		t.file.Close()
	}
}

// Option to pass to the using Functional Options
type Option func(*Tar) error

// SkipDirGlob is a function used by users to set options.
func SkipDirGlob(s string) Option {
	return func(t *Tar) error {
		if s != "" {
			if pattern, err := glob.New(s); err != nil {
				return fmt.Errorf("Invalid SkipDir glob pattern '%s', %s", s, err.Error())
			} else {
				t.SkipDirGlob = pattern
			}
		}
		return nil
	}
}

// SkipFileGlob is a function used by users to set options.
func SkipFileGlob(s string) Option {
	return func(t *Tar) error {
		if s != "" {
			if pattern, err := glob.New(s); err != nil {
				return fmt.Errorf("Invalid SkipFile glob pattern '%s', %s", s, err.Error())
			} else {
				t.SkipFileGlob = pattern
			}
		}
		return nil
	}
}

// FileGlob is a function used by users to set options.
func FileGlob(s string) Option {
	return func(t *Tar) error {
		if s != "" {
			if pattern, err := glob.New(s); err != nil {
				return fmt.Errorf("Invalid File glob pattern '%s', %s", s, err.Error())
			} else {
				t.FileGlob = pattern
			}
		}
		return nil
	}
}

// DirGlob is a function used by users to set options.
func DirGlob(s string) Option {
	return func(t *Tar) error {
		if s != "" {
			if pattern, err := glob.New(s); err != nil {
				return fmt.Errorf("Invalid Dir glob pattern '%s', %s", s, err.Error())
			} else {
				t.DirGlob = pattern
			}
		}
		return nil
	}
}

func (t *Tar) Add(ctx context.Context, src, dstpath string, opts ...Option) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ctx = ctx
	t.srcPath = src
	t.dstPath = "."
	if dstpath != "" {
		t.dstPath = dstpath
	}
	// call option functions on instance to set options on it
	for _, opt := range opts {
		if err := opt(t); err != nil {
			t.log.Error(err)
			return err
		}
	}
	if _, err := os.Stat(src); err == nil {
		return filepath.Walk(src, t.scan)
	} else {
		return err
	}
}

func (t *Tar) AddFile(f fs.File, path string, mode os.FileMode) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	i, err := f.Stat()
	if err != nil {
		err = fmt.Errorf("Cannot stat FS object '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	header, err := tar.FileInfoHeader(i, i.Name())
	if err != nil {
		err = fmt.Errorf("Cannot get tar header for file '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	header.Mode = int64(mode.Perm())
	header.Name = filepath.Join(t.BasePath, path)
	if err := t.tw.WriteHeader(header); err != nil {
		err = fmt.Errorf("Cannot store tar header for file '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	bytes, err := io.Copy(t.tw, f)
	if err != nil {
		err = fmt.Errorf("Cannot tar file '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	t.log.Debugf("Tar file '%s': %d bytes", path, bytes)
	return err
}

func (t *Tar) scan(p string, i os.FileInfo, err error) error {
	if err != nil {
		err = fmt.Errorf("Cannot scan path for tar, %s", err.Error())
		t.log.Error(err)
		return err
	}
	if i.IsDir() {
		if t.SkipDirGlob != nil && t.SkipDirGlob.MatchString(p) {
			t.log.Debugf("Skipping folder due to glob '%s': %s", t.SkipDirGlob.String(), p)
			return filepath.SkipDir
		} else if t.DirGlob != nil && !t.DirGlob.MatchString(p) {
			t.log.Debugf("Skipping folder due to not matching glob '%s': %s", t.DirGlob.String(), p)
			return filepath.SkipDir
		}
	} else if i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 {
		if t.SkipFileGlob != nil && t.SkipFileGlob.MatchString(p) {
			t.log.Debugf("Skipping file due to glob '%s': %s", t.SkipFileGlob.String(), p)
			return nil
		} else if t.FileGlob != nil && !t.FileGlob.MatchString(p) {
			t.log.Debugf("Skipping file due to not matching glob '%s': %s", t.FileGlob.String(), p)
			return nil
		}
	} else {
		t.log.Debugf("Skipping non regular file: %s", p)
		return nil
	}
	select {
	case <-t.ctx.Done():
		err = fmt.Errorf("Cancelled by context")
		t.log.Error(err)
		return err
	default:
		return t.tarFile(p, i)
	}
}

func (t *Tar) tarFile(path string, i os.FileInfo) error {
	header, err := tar.FileInfoHeader(i, i.Name())
	if err != nil {
		err = fmt.Errorf("Cannot get tar header for file '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	// Remove relative paths
	name := strings.TrimPrefix(path, t.srcPath)
	if name == "" {
		name = filepath.Base(path)
	}
	header.Name = filepath.Join(t.dstPath, name)
	// Add base
	header.Name = filepath.Join(t.BasePath, header.Name)
	if err := t.tw.WriteHeader(header); err != nil {
		err = fmt.Errorf("Cannot store tar header for file '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	if i.IsDir() {
		t.log.Debugf("Adding directory '%s'", path)
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		err = fmt.Errorf("Cannot open file '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	defer file.Close()
	bytes, err := io.Copy(t.tw, file)
	if err != nil {
		err = fmt.Errorf("Cannot tar file '%s': %s", path, err.Error())
		t.log.Error(err)
		return err
	}
	t.log.Debugf("Tar file '%s': %d bytes", path, bytes)
	return err
}

func (t *Tar) UnTar(ctx context.Context, reader io.Reader) error {
	tarReader := tar.NewReader(reader)
	for {
		select {
		case <-t.ctx.Done():
			err := fmt.Errorf("Cancelled by context")
			t.log.Error(err)
			return err
		default:
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				err = fmt.Errorf("Cannot untar: %s", err.Error())
				t.log.Error(err)
				return err
			}
			path := filepath.Join(t.BasePath, header.Name)
			info := header.FileInfo()
			// Alternative:
			// switch header.Typeflag
			// 	case tar.TypeDir: ...
			// 	case tar.TypeReg: ...
			if info.IsDir() {
				if err = os.MkdirAll(path, info.Mode()); err != nil {
					err = fmt.Errorf("Cannot create directory '%s' with mode '%s': %s", path, info.Mode().String(), err.Error())
					t.log.Error(err)
					return err
				}
				t.log.Debugf("Created folder '%s' with mode '%s'", path, info.Mode().String())
				continue
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if err != nil {
				err = fmt.Errorf("Cannot open '%s' for writing: %s", path, err.Error())
				t.log.Error(err)
				return err
			}
			defer file.Close()
			bytes, err := io.Copy(file, tarReader)
			if err != nil {
				err = fmt.Errorf("Cannot write to '%s': %s", path, err.Error())
				t.log.Error(err)
				return err
			}
			t.log.Debugf("Extracted file '%s': %d bytes", path, bytes)
			return err
		}
	}
}

func TarFile(srcpath, tarball string) error {
	l := log.StandardLogger()
	t, err := NewTarFile(tarball, l)
	if err != nil {
		return err
	}
	defer t.Close()
	ctx := context.Background()
	return t.Add(ctx, srcpath, ".")
}

func UnTarFile(tarball, dstpath string) error {
	l := log.StandardLogger()
	t := NewTar(dstpath, l)
	defer t.Close()
	reader, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer reader.Close()
	ctx := context.Background()
	return t.UnTar(ctx, reader)
}
