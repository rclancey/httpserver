package httpserver

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var envre1 = regexp.MustCompile(`\$([A-Za-z0-9_]+)`)
var envre2 = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)
func envRepl1(m string) string {
	k := m[1:]
	return os.Getenv(k)
}
func envRepl2(m string) string {
	k := m[2:len(m)-1]
	return os.Getenv(k)
}

func EnvEval(s string) string {
	xs := envre1.ReplaceAllStringFunc(s, envRepl1)
	xs = envre2.ReplaceAllStringFunc(xs, envRepl2)
	return xs
}

func makeRootAbs(serverRoot string, fn string) (string, error) {
	fn = EnvEval(fn)
	if fn == "" {
		return "", nil
	}
	if isRootRel(fn) {
		return filepath.Clean(filepath.Join(serverRoot, fn)), nil
	}
	return filepath.Abs(filepath.Clean(fn))
}

func isRootRel(path string) bool {
	if strings.HasPrefix(path, string(filepath.Separator)) {
		return false
	}
	if strings.HasPrefix(path, "." + string(filepath.Separator)) {
		return false
	}
	return true
}

func checkReadableFile(fn string) error {
	if fn == "" {
		return nil
	}
	st, err := os.Stat(fn)
	if err != nil {
		return errors.Wrap(err, "can't stat file " + fn)
	}
	if st.IsDir() {
		return errors.Errorf("filename %s points to a directory", fn)
	}
	f, err := os.Open(fn)
	if err != nil {
		return errors.Wrap(err, "can't open file " + fn)
	}
	f.Close()
	return nil
}

func checkWritableDir(dn string) error {
	if dn == "" {
		return nil
	}
	st, err := os.Stat(dn)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dn, 0775)
			if err != nil {
				return errors.Wrap(err, "can't make dirs " + dn)
			}
			return nil
		} else {
			return errors.Wrap(err, "can't stat dir " + dn)
		}
	}
	if !st.IsDir() {
		return errors.Errorf("filename %s doesn't point to a directory", dn)
	}
	return nil
}

