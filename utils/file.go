package utils

import (
	"io/ioutil"
	"os"
	opath "path"
)

func DelGitFile(path string) error {
	fileInfo, err := os.Stat(path)
	if os.IsNotExist(err) {
		// ignore file not exist
		return nil
	}
	if fileInfo.IsDir() {
		//iterator dir and delete files
		fileInfos, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		for _, d := range fileInfos {
			fullPath := opath.Join(path, d.Name())
			err := os.RemoveAll(fullPath)
			if err != nil {
				return err
			}
		}
	} else {
		err := os.RemoveAll(path)
		if err != nil {
			return err
		}
	}
	return nil
}
