package cgroups

import (
	"testing"
)

func Test_cgroups_Load(t *testing.T) {
	testdata := map[string]struct {
		cgroup         string
		expectedFolder string
		expectedFile   string
		expectedValues []string
	}{
		"mem": {
			cgroup: `
			{
				"folders": {
					"memory": {
						"files": {
							"memory.limit_in_bytes": [
								"209715200"
							]
						}
					}
				}
			}
			`,
			expectedFolder: "memory",
			expectedFile:   "memory.limit_in_bytes",
			expectedValues: []string{"209715200"},
		},
	}

	for name, test := range testdata {
		t.Run(name, func(t *testing.T) {
			c, err := LoadCGroups("testparent", []byte(test.cgroup))
			if err != nil {
				t.Fatal(err)
			}

			if c.Folders[test.expectedFolder].Files == nil {
				t.Fatalf("expected cgroup folder to be set")
			}

			if c.Folders[test.expectedFolder].Files[test.expectedFile] == nil {
				t.Fatalf("expected cgroup file to be set")
			}

			if len(c.Folders[test.expectedFolder].Files[test.expectedFile]) != len(test.expectedValues) {
				t.Fatalf("expected %d values, got %d", len(test.expectedValues), len(c.Folders[test.expectedFolder].Files[test.expectedFile]))
			}

			for i, expectedValue := range test.expectedValues {
				if c.Folders[test.expectedFolder].Files[test.expectedFile][i] != expectedValue {
					t.Fatalf("expected %s, got %s", expectedValue, c.Folders[test.expectedFolder].Files[test.expectedFile][i])
				}
			}
		})
	}
}
