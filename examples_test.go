package dimail

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// clientCallRE matches a method invoked on the `client` receiver in the Ruby
// examples, e.g. `client.get_domain(` -> "get_domain".
var clientCallRE = regexp.MustCompile(`client\.([a-z_][a-z0-9_]*)`)

// TestRubyExamplesReferenceRealOperations parses every examples/*.rb file and
// asserts that each operation it invokes on the client is a real, dispatchable
// operation of the underlying go-dimail client. This keeps the documented Ruby
// surface from drifting away from the generated API.
func TestRubyExamplesReferenceRealOperations(t *testing.T) {
	files, err := filepath.Glob("examples/*.rb")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no examples/*.rb files found")
	}

	ops := map[string]bool{}
	for _, name := range NewSession().Methods() {
		ops[name] = true
	}

	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		matches := clientCallRE.FindAllStringSubmatch(string(src), -1)
		if len(matches) == 0 {
			t.Errorf("%s: no client operations found", filepath.Base(f))
		}
		for _, m := range matches {
			if !ops[m[1]] {
				t.Errorf("%s: client.%s is not a dimail operation", filepath.Base(f), m[1])
			}
		}
	}
}
