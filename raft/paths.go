package raft

import (
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
	err = os.MkdirAll(ProjectDir, 0755)
	if err != nil {
		log.Fatalf("error creating project directory; %v\n", err)
	}
}

func GetVotersFile(address string) string {
	return filepath.Join(ProjectDir, address, "raft.votes")
}

func GetLogFile(address string) string {
	return filepath.Join(ProjectDir, address, "raft.logs")
}
