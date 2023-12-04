package cgroups

import (
	"io/fs"
	"path/filepath"
)

// GetAllPids returns all pids from the cgroup identified by path, and all its
// sub-cgroups.
func GetAllPids(path string) ([]int, error) {
	var pids []int
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, iErr error) error {
		if iErr != nil {
			return iErr
		}
		if !d.IsDir() {
			return nil
		}
		cPids, err := readTaskFile(p, CgroupProcesses)
		if err != nil {
			return err
		}
		pids = append(pids, cPids...)
		return nil
	})
	return pids, err
}

// GetAllTids returns all tids from the cgroup identified by path, and all its
// sub-cgroups.
func GetAllTids(path string) ([]int, error) {
	var tids []int
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, iErr error) error {
		if iErr != nil {
			return iErr
		}
		if !d.IsDir() {
			return nil
		}
		cTids, err := readTaskFile(p, CgroupThreads)
		if err != nil {
			return err
		}
		tids = append(tids, cTids...)
		return nil
	})
	return tids, err
}
