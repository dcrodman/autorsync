// dirsync is a simple utility for monitoring changes to a directory and then
// using the system's rsync to update the files on the target.
package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	configFile = flag.String("config", ".autorsync", "Config file")
	rsync      = flag.String("rsync", "/usr/bin/rsync", "rsync executable to use")

	needsRsync      map[*mapping]bool
	needsRsyncMutex sync.Mutex
)

type settings struct {
	Interval  string
	RsyncArgs []string `json:"rsync_args"`

	refreshInterval time.Duration
}

type mapping struct {
	Source     string
	Target     string
	Exclusions []string
}

type config struct {
	Settings *settings
	Mappings []*mapping
}

func main() {
	flag.Parse()

	config := readConfig(*configFile)
	needsRsync = make(map[*mapping]bool)

	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()

	for _, mapping := range config.Mappings {
		log.Printf("syncing %s to %s\n", mapping.Source, mapping.Target)
		watchFilesInDirectory(watcher, mapping.Source, mapping.Exclusions)

		needsRsync[mapping] = false
	}

	var err error
	config.Settings.refreshInterval, err = time.ParseDuration(config.Settings.Interval)
	if err != nil {
		log.Fatal("failed to parse interval:", err)
	}

	go startRsyncLoop(config)
	waitForSyncEvents(config.Mappings, watcher.Events, watcher.Errors)
}

func readConfig(configFile string) *config {
	var conf config

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal("failed to open config file: ", err)
	}

	if err := json.Unmarshal(data, &conf); err != nil {
		log.Fatal("failed to parse config file: ", err)
	}

	for _, mapping := range conf.Mappings {
		mapping.Source = os.ExpandEnv(mapping.Source)
		mapping.Target = os.ExpandEnv(mapping.Target)

		// Automatically ignore the autorsync config file.
		mapping.Exclusions = append(mapping.Exclusions, configFile)
	}

	return &conf
}

// Traverse the specified path, adding any files and subdirectories to the watcher
// that are not in the list of exclusions.
func watchFilesInDirectory(watcher *fsnotify.Watcher, basePath string, exclusions []string) error {
	// path is always prefixed with the top-level directory path from mapper.Source (basePath), so
	// to make comparison simnple the excluded dirs are made relative to the base path.
	normalizedPathExclusions := make([]string, len(exclusions))
	for i, exclusion := range exclusions {
		if strings.HasPrefix(exclusion, basePath) {
			normalizedPathExclusions[i] = exclusion
		} else {
			normalizedPathExclusions[i] = filepath.Join(basePath, exclusion)
		}
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}

		for _, excludedPath := range normalizedPathExclusions {
			if strings.HasPrefix(path, excludedPath) {
				return nil
			}
		}

		return watcher.Add(path)
	}

	if err := filepath.Walk(basePath, walkFn); err != nil {
		log.Fatal("error while traversing directory: ", err)
	}

	return nil
}

// Wait for events from fsnotify on any of the files we watched.
func waitForSyncEvents(mappings []*mapping, events chan fsnotify.Event, errors chan error) {
	for {
		select {
		case event := <-events:
			log.Println("[event] detected change to", event.Name)
			needsRsyncMutex.Lock()

			for _, mapping := range mappings {
				if strings.HasPrefix(event.Name, mapping.Source) {
					needsRsync[mapping] = true
					break
				}
			}

			needsRsyncMutex.Unlock()
		case err := <-errors:
			log.Println("[error]", err)
		}
	}
}

// Listen for requests to update directories and update any affected targets.
func startRsyncLoop(config *config) {
	c := time.Tick(config.Settings.refreshInterval)
	for _ = range c {
		needsRsyncMutex.Lock()

		for mapping, needsSync := range needsRsync {
			if needsSync {
				runRsync(config, mapping)
				needsRsync[mapping] = false
			}
		}

		needsRsyncMutex.Unlock()
	}
}

// Build and run the underlying rsync command to update mapping.Target with the
// contents of mapping.Source.
func runRsync(config *config, mapping *mapping) {
	args := make([]string, 0)
	args = append(args, "-avzh")

	for _, arg := range config.Settings.RsyncArgs {
		args = append(args, os.ExpandEnv(arg))
	}

	for _, exclusion := range mapping.Exclusions {
		args = append(args, "--exclude="+exclusion)
	}

	args = append(args, mapping.Source, mapping.Target)
	rsyncCommand := exec.Command(*rsync, args...)

	log.Println(rsyncCommand.String())

	if output, err := rsyncCommand.Output(); err != nil {
		log.Println("[error] rsync failed:", string(err.(*exec.ExitError).Stderr))
	} else {
		log.Println(string(output))
	}
}
