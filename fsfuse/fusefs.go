// Copyright 2023 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package fsfuse exposes an fs.FS as a FUSE server.
package fsfuse

import (
	"context"
	"io"
	"io/fs"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type FS struct {
	*fuseutil.NotImplementedFileSystem
	fsys     fs.FS
	cacheDur time.Duration
	uid, gid uint32

	inodeSeq   uint64
	handleSeq  uint64
	generation uint64 // GenerationNumber - must be incremeneted on each inode reuse (or inde removal)

	mu             sync.RWMutex
	inodePaths     map[fuseops.InodeID]string
	pathInodes     map[string]fuseops.InodeID
	inodeRefCounts map[fuseops.InodeID]uint32
	files          map[fuseops.HandleID]fs.File
}

const DefaultCacheDur = 356 * 24 * time.Hour

// NewFS returns a fuser.Server for the given fs.FS.
//
// If cacheDur < 0 then the caching will be disabled;
// if cacheDur == 0 then the default 1 year will be used.
func NewFS(fsys fs.FS, uid, gid uint32, cacheDur time.Duration) fuse.Server {
	if cacheDur < 0 {
		cacheDur = 0
	} else if cacheDur == 0 {
		cacheDur = DefaultCacheDur
	}
	return fuseutil.NewFileSystemServer(&FS{
		fsys: fsys, uid: uid, gid: gid, cacheDur: cacheDur,
	})
}

func (f *FS) getPathInode(fn string) fuseops.InodeID {
	f.mu.RLock()
	inode, ok := f.pathInodes[fn]
	f.mu.RUnlock()
	if ok {
		return inode
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if inode, ok = f.pathInodes[fn]; ok {
		return inode
	}
	i := fuseops.InodeID(atomic.AddUint64(&f.inodeSeq, 1))
	f.pathInodes[fn] = i
	f.inodePaths[i] = fn
	return i
}

func (f *FS) infoAttributes(fi fs.FileInfo) fuseops.InodeAttributes {
	return fuseops.InodeAttributes{
		Size:  uint64(fi.Size()),
		Mode:  fi.Mode(),
		Atime: fi.ModTime(),
		Mtime: fi.ModTime(),
		Ctime: fi.ModTime(),
		Uid:   f.uid, Gid: f.gid,
	}
}

func (f *FS) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	f.mu.RLock()
	fn := path.Join(f.inodePaths[op.Parent], op.Name)
	f.mu.RUnlock()
	file, err := f.fsys.Open(fn)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	file.Close()
	if err != nil {
		return err
	}
	op.Entry = fuseops.ChildInodeEntry{
		Child:      f.getPathInode(fn),
		Generation: fuseops.GenerationNumber(f.generation),
		Attributes: f.infoAttributes(fi),
	}
	f.inodeRefCounts[op.Entry.Child]++
	if f.cacheDur != 0 {
		op.Entry.AttributesExpiration = time.Now().Add(f.cacheDur)
		op.Entry.EntryExpiration = op.Entry.AttributesExpiration
	}
	return nil
}

func (f *FS) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	f.mu.RLock()
	path := f.inodePaths[op.Inode]
	f.mu.RUnlock()
	file, err := f.fsys.Open(path)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	file.Close()
	if err != nil {
		return err
	}
	op.Attributes = f.infoAttributes(fi)
	if f.cacheDur != 0 {
		op.AttributesExpiration = time.Now().Add(f.cacheDur)
	}
	return nil
}

func (f *FS) forgetInode(inode fuseops.InodeID, N uint64) error {
	if N == 0 {
		N = 1
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if rc, ok := f.inodeRefCounts[inode]; ok {
		if uint64(rc) > N {
			f.inodeRefCounts[inode] = rc - uint32(N)
		} else {
			delete(f.pathInodes, f.inodePaths[inode])
			delete(f.inodePaths, inode)
			delete(f.inodeRefCounts, inode)
		}
	}
	return nil
}

func (f *FS) ForgetInode(ctx context.Context, op *fuseops.ForgetInodeOp) error {
	f.forgetInode(op.Inode, 1)
	return nil
}

func (f *FS) BatchForget(ctx context.Context, op *fuseops.BatchForgetOp) error {
	for _, e := range op.Entries {
		f.forgetInode(e.Inode, e.N)
	}
	return nil
}

func (f *FS) openFile(inode fuseops.InodeID) (fuseops.HandleID, error) {
	f.mu.RLock()
	path, ok := f.inodePaths[inode]
	f.mu.RUnlock()
	if !ok {
		return 0, fuse.ENOENT
	}
	file, err := f.fsys.Open(path)
	if err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	handle := fuseops.HandleID(atomic.AddUint64(&f.handleSeq, 1))
	f.files[handle] = file
	return handle, nil
}

func (f *FS) OpenDir(ctx context.Context, op *fuseops.OpenDirOp) error {
	handle, err := f.openFile(op.Inode)
	if err != nil {
		return err
	}
	op.Handle = handle
	return nil
}

func (f *FS) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	f.mu.RLock()
	dn := f.inodePaths[op.Inode]
	f.mu.RUnlock()
	dis, err := fs.ReadDir(f.fsys, dn)
	if op.Offset < fuseops.DirOffset(len(dis)) {
		dis = dis[op.Offset:]
	}
	for i, di := range dis {
		inode := f.getPathInode(path.Join(dn, di.Name()))
		typ := fuseutil.DT_File
		if di.Type().IsDir() {
			typ = fuseutil.DT_Directory
		}
		fuseutil.WriteDirent(op.Dst, fuseutil.Dirent{
			Offset: op.Offset + fuseops.DirOffset(i),
			Inode:  inode,
			Name:   di.Name(),
			Type:   typ,
		})
	}
	return err
}
func (f *FS) ReleaseDirHandle(ctx context.Context, op *fuseops.ReleaseDirHandleOp) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if file, ok := f.files[op.Handle]; ok {
		file.Close()
		delete(f.files, op.Handle)
	}
	return nil
}

func (f *FS) OpenFile(ctx context.Context, op *fuseops.OpenFileOp) error {
	file, err := f.openFile(op.Inode)
	if err != nil {
		return err
	}
	op.Handle = file
	op.KeepPageCache = true
	return err
}

func (f *FS) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	f.mu.RLock()
	file, ok := f.files[op.Handle]
	path := f.inodePaths[op.Inode]
	f.mu.RUnlock()
	if !ok {
		return fuse.EINVAL
	}
	var err error
	if spec, ok := file.(io.ReaderAt); ok {
		op.BytesRead, err = spec.ReadAt(op.Dst, op.Offset)
	} else if spec, ok := file.(io.Seeker); ok {
		if _, err = spec.Seek(op.Offset, io.SeekStart); err != nil {
			return err
		}
		op.BytesRead, err = file.Read(op.Dst)
	} else if spec, ok := f.fsys.(fs.ReadFileFS); ok {
		var data []byte
		data, err = spec.ReadFile(path)
		op.BytesRead = copy(op.Dst, data[op.Offset:])
	} else {
		file, err := f.fsys.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if op.Offset != 0 {
			if _, err = io.CopyBuffer(io.Discard, io.LimitReader(file, op.Offset), op.Dst); err != nil {
				return err
			}
		}
		op.BytesRead, err = file.Read(op.Dst)
	}

	// Don't return EOF errors; we just indicate EOF to fuse using a short read.
	if err == io.EOF {
		return nil
	}
	return err
}

func (f *FS) ReleaseFileHandle(ctx context.Context, op *fuseops.ReleaseFileHandleOp) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if file, ok := f.files[op.Handle]; ok {
		file.Close()
		delete(f.files, op.Handle)
	}
	return nil
}
