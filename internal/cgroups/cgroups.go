package cgroups

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func LimitResources(parent string, config []byte) error {
	cg, err := LoadCGroups(parent, config)
	if err != nil {
		return err
	}

	return cg.Write()
}

// cgroupPath is the path to the default cgroup path
// on a Linux system.
var cgroupPath = "/sys/fs/cgroup"

// CGroups represents a set of cgroups directories.
type CGroups struct {
	Folders map[string]CGroupFiles `json:"folders"`

	procID       int    `json:"-"`
	parentFolder string `json:"-"`
}

// CGroupFiles represents a set of files in a cgroup folder.
type CGroupFiles struct {
	Files map[string][]string `json:"files"`
}

// LoadCGroups loads a set of cgroups from a JSON configuration.
// The JSON configuration follows following format:
/*
{
    "folders": {
        "memory": {
            "files": {
                "memory.limit_in_bytes": [
                    "209715200"
                ]
            }
        },
        "cpu,cpuacct": {
            "files": {
                "cpu.cfs_quota_us": [
                    "100000"
                ]
            }
        },
        "blkio": {
            "files": {
                "blkio.throttle.write_bps_device": [
                    "8: 0 10485760"
                ]
            }
        }
    }
}
*/
func LoadCGroups(parentFolder string, config []byte) (CGroups, error) {
	c := CGroups{}

	err := json.Unmarshal(config, &c)
	if err != nil {
		panic(err)
	}

	return c, nil
}

// Write creates cgroup subfolders and writes the values to the files
// in the cgroup using the configuration loaded into `c`.
func (c CGroups) Write() error {
	for cgRoot, files := range c.Folders {
		// Create directories if they don't exist.
		cgPath := filepath.Join(
			cgroupPath,
			cgRoot,
			c.parentFolder,
			strconv.Itoa(c.procID),
		)

		err := os.MkdirAll(cgPath, 0755)
		if err != nil {
			return err
		}

		// Write values to the cgroup files for this group.
		for file, values := range files.Files {
			err = os.WriteFile(
				filepath.Join(cgPath, file),
				[]byte(strings.Join(values, "\n")),
				0600,
			)
			if err != nil {
				return err
			}
		}

		// Setup cleanup for this cgroup.
		err = os.WriteFile(
			filepath.Join(cgPath, "notify_on_release"),
			[]byte("1"),
			0600,
		)
		if err != nil {
			return err
		}

		// Add the current process to the cgroup.
		err = os.WriteFile(
			filepath.Join(cgPath, "cgroup.procs"),
			[]byte(strconv.Itoa(c.procID)),
			0600,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// Clean attempts to cleanup the cgroup folders created by `c`.
func (c CGroups) Clean(parent string) {
	for cgRoot := range c.Folders {
		cgPath := filepath.Join(cgroupPath, cgRoot, c.parentFolder)

		_ = os.RemoveAll(cgPath)
		// TODO: Try to correctly remove cgroup folders.
		// if err != nil {
		// 	fmt.Println(err)
		// }
	}
}
