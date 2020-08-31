package operations

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// Creates an OptionResume that will be passed to Put/Upload
func createResumeOpt(ctx context.Context, f fs.Fs, remote string, src fs.Object) (resumeOpt *fs.OptionResume) {
	ci := fs.GetConfig(ctx)
	resumeOpt = &fs.OptionResume{ID: "", Pos: 0, SetID: createSetID(ctx, f, remote, src)}
	if ci.ResumeLarger >= 0 {
		cacheName := filepath.Join(config.CacheDir, "resume", f.Name(), f.Root(), remote)
		resumeID, hashName, hashState, attemptResume := readResumeCache(ctx, f, src, cacheName)
		if attemptResume {
			fs.Debugf(f, "Existing resume cache file found: %s. A resume will now be attmepted.", cacheName)
			position, resumeErr := f.Features().Resume(ctx, remote, resumeID, hashName, hashState)
			if resumeErr == nil && position > int64(ci.ResumeLarger) {
				(*resumeOpt).Pos = position
			}
		}
	}
	return resumeOpt
}

// SetID will be called by backend's Put/Update function if the object's upload
// could be resumed upon failure
//
// SetID takes the passed resume ID, hash state, hash name and Fingerprint of the object and stores it in
// --cache-dir so that future Copy operations can resume the upload if it fails
func createSetID(ctx context.Context, f fs.Fs, remote string, src fs.Object) func(ID, hashName, hashState string) (err error) {
	ci := fs.GetConfig(ctx)
	cacheCleaned := false
	return func(ID, hashName, hashState string) (err error) {
		rootCacheDir := filepath.Join(config.CacheDir, "resume")
		// Get the Fingerprint of the src object so that future Copy operations can ensure the
		// object hasn't changed before resuming an upload
		fingerprint := fs.Fingerprint(ctx, src, true)
		data, err := marshalResumeJSON(ctx, fingerprint, ID, hashName, hashState)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal data JSON")
		}
		if len(data) < int(ci.MaxResumeCacheSize) {
			// Each remote will have its own dir for cached resume files
			dirPath := filepath.Join(rootCacheDir, f.Name(), f.Root())
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				return errors.Wrapf(err, "failed to create cache directory %v", dirPath)
			}
			// Write resume data to disk
			cachePath := filepath.Join(dirPath, remote)
			cacheFile, err := os.Create(cachePath)
			if err != nil {
				return errors.Wrapf(err, "failed to create cache file %v", cachePath)
			}
			defer func() {
				_ = cacheFile.Close()
			}()
			_, errWrite := cacheFile.Write(data)
			if err != nil {
				return errors.Wrapf(errWrite, "failed to write JSON to file")
			}
		}
		if !cacheCleaned {
			if err := cleanCache(ctx, rootCacheDir); err != nil {
				return errors.Wrapf(err, "failed to clean resume cache")
			}
		}
		cacheCleaned = true
		return nil
	}
}

// cleanCache checks the size of the resume cache and removes the oldest resume files if more than limit
func cleanCache(ctx context.Context, rootCacheDir string) error {
	ci := fs.GetConfig(ctx)
	var paths []string
	pathsWithInfo := make(map[string]os.FileInfo)
	totalCacheSize := int64(0)
	walkErr := filepath.Walk(rootCacheDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				// Empty subdirectories in the resume cache dir can be removed
				removeErr := os.Remove(path)
				if err != nil && !os.IsNotExist(removeErr) {
					return errors.Wrapf(err, "failed to remove empty subdirectory: %s", path)
				}
				return nil
			}
			paths = append(paths, path)
			pathsWithInfo[path] = info
			totalCacheSize += info.Size()
			return nil
		})
	if walkErr != nil {
		return errors.Wrapf(walkErr, "error walking through cache when cleaning cache dir")
	}
	if totalCacheSize > int64(ci.MaxResumeCacheSize) {
		sort.Slice(paths, func(i, j int) bool {
			return pathsWithInfo[paths[i]].ModTime().Before(pathsWithInfo[paths[j]].ModTime())
		})
		for _, p := range paths {
			if totalCacheSize < int64(ci.MaxResumeCacheSize) {
				break
			}
			if err := os.Remove(p); err != nil {
				return errors.Wrapf(err, "error removing oldest cache file: %s", p)
			}
			totalCacheSize -= pathsWithInfo[p].Size()
			fs.Debugf(p, "Successfully removed oldest cache file")
		}
	}
	return nil
}

// readResumeCache checks to see if a resume ID has been cached for the source object.
// If it finds one it returns it along with true to signal a resume can be attempted
func readResumeCache(ctx context.Context, f fs.Fs, src fs.Object, cacheName string) (resumeID, hashName, hashState string, attemptResume bool) {
	existingCacheFile, statErr := os.Open(cacheName)
	defer func() {
		_ = existingCacheFile.Close()
	}()
	if !os.IsNotExist(statErr) {
		rawData, readErr := ioutil.ReadAll(existingCacheFile)
		if readErr == nil {
			existingFingerprint, resumeID, hashName, hashState, unmarshalErr := unmarshalResumeJSON(ctx, rawData)
			if unmarshalErr != nil {
				fs.Debugf(f, "Failed to unmarshal Resume JSON: %s. Resume will not be attempted.", unmarshalErr.Error())
			} else if existingFingerprint != "" {
				// Check if the src object has changed by comparing new Fingerprint to Fingerprint in cache file
				fingerprint := fs.Fingerprint(ctx, src, true)
				if existingFingerprint == fingerprint {
					return resumeID, hashName, hashState, true
				}
			}
		}
	}
	return "", "", "", false
}

// Struct for storing resume info in cache
type resumeJSON struct {
	Fingerprint string `json:"fprint"`
	ID          string `json:"id"`
	HashName    string `json:"hname"`
	HashState   string `json:"hstate"`
}

func marshalResumeJSON(ctx context.Context, fprint, id, hashName, hashState string) ([]byte, error) {
	resumedata := resumeJSON{
		Fingerprint: fprint,
		ID:          id,
		HashName:    hashName,
		HashState:   hashState,
	}
	data, err := json.Marshal(&resumedata)
	return data, err
}

func unmarshalResumeJSON(ctx context.Context, data []byte) (fprint, id, hashName, hashState string, err error) {
	var resumedata resumeJSON
	err = json.Unmarshal(data, &resumedata)
	if err != nil {
		return "", "", "", "", err
	}
	return resumedata.Fingerprint, resumedata.ID, resumedata.HashName, resumedata.HashState, nil
}
