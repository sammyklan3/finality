package raft

import (
	"errors"
	"log"
	"os"
	"path/filepath"
)

var (
	ProjectDir string
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("error getting users home directory; %v\n", err)
	}

	ProjectDir = filepath.Join(homeDir, ".finality")
	err = os.Mkdir(ProjectDir, 0755)
	if err != nil {
		err_dir_exists := errors.Is(err, os.ErrExist)
		if !err_dir_exists {
			log.Fatalf("error creating project directory; %v\n", err)
		}
	}
}
