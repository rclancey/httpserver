package httpserver

import (
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

var truish = true
var falsish = false

func ScanETag(s string) (etag string, remain string) {
	s = textproto.TrimString(s)
	start := 0
	if strings.HasPrefix(s, "W/") {
		start = 2
	}
	if len(s[start:]) < 2 || s[start] != '"' {
		return "", ""
	}
	// ETag is either W/"text" or "text".
	// See RFC 7232 2.3.
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		switch {
		// Character values allowed in ETags.
		case c == 0x21 || c >= 0x23 && c <= 0x7E || c >= 0x80:
		case c == '"':
			return s[:i+1], s[i+1:]
		default:
			return "", ""
		}
	}
	return "", ""
}

func etagStrongMatch(a, b string) bool {
	return a == b && a != "" && a[0] == '"'
}

func etagWeakMatch(a, b string) bool {
	return strings.TrimPrefix(a, "W/") == strings.TrimPrefix(b, "W/")
}

func CheckIfMatch(r *http.Request, etag string) *bool {
	im := r.Header.Get("If-Match")
	if im == "" {
		return nil
	}
	for {
		im = textproto.TrimString(im)
		if len(im) == 0 {
			break
		}
		if im[0] == ',' {
			im = im[1:]
			continue
		}
		if im[0] == '*' {
			return &truish
		}
		retag, remain := ScanETag(im)
		if retag == "" {
			break
		}
		if etagStrongMatch(etag, retag) {
			return &truish
		}
		im = remain
	}
	return &falsish
}

func CheckIfNoneMatch(r *http.Request, etag string) *bool {
	inm := r.Header.Get("If-None-Match")
	if inm == "" {
		return nil
	}
	buf := inm
	for {
		buf = textproto.TrimString(buf)
		if len(buf) == 0 {
			break
		}
		if buf[0] == ',' {
			buf = buf[1:]
		}
		if buf[0] == '*' {
			return &falsish
		}
		retag, remain := ScanETag(buf)
		if retag == "" {
			break
		}
		if etagWeakMatch(etag, retag) {
			return &falsish
		}
		buf = remain
	}
	return &truish
}

var unixEpoch = time.Unix(0, 0)

func isZeroTime(t time.Time) bool {
	return t.IsZero() || t.Equal(unixEpoch)
}

func CheckIfModifiedSince(r *http.Request, modTime time.Time) *bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return nil
	}
	ims := r.Header.Get("If-Modified-Since")
	if ims == "" || isZeroTime(modTime) {
		return nil
	}
	t, err := http.ParseTime(ims)
	if err != nil {
		return nil
	}
	if modTime.Before(t.Add(time.Second)) {
		return &falsish
	}
	return &truish
}

func CheckIfUnmodifiedSince(r *http.Request, modTime time.Time) *bool {
	ius := r.Header.Get("If-Unmodified-Since")
	if ius == "" || isZeroTime(modTime) {
		return nil
	}
	if t, err := http.ParseTime(ius); err == nil {
		if modTime.Before(t.Add(time.Second)) {
			return &truish
		}
		return &falsish
	}
	return nil
}

func CheckIfRange(r *http.Request, modTime time.Time, etag string) *bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return nil
	}
	ir := r.Header.Get("If-Range")
	if ir == "" {
		return nil
	}
	retag, _ := ScanETag(ir)
	if retag != "" {
		if etagStrongMatch(etag, retag) {
			return &truish
		} else {
			return &falsish
		}
	}
	if isZeroTime(modTime) {
		return &falsish
	}
	t, err := http.ParseTime(ir)
	if err != nil {
		return &falsish
	}
	if t.Unix() == modTime.Unix() {
		return &truish
	}
	return &falsish
}

func CheckPreconditions(r *http.Request, modTime time.Time, etag string) error {
	ch := CheckIfMatch(r, etag)
	if ch == nil {
		ch = CheckIfUnmodifiedSince(r, modTime)
	}
	if ch != nil && *ch == false {
		return PreconditionFailed
	}
	ch = CheckIfNoneMatch(r, etag)
	if ch == nil {
		ch = CheckIfModifiedSince(r, modTime)
		if ch != nil && *ch == false {
			return NotModified
		}
	} else if *ch == false {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			return NotModified
		} else {
			return PreconditionFailed
		}
	}
	rh := r.Header.Get("Range")
	if rh != "" {
		ch = CheckIfRange(r, modTime, etag)
		if ch != nil && *ch == false {
			return Conflict
		}
	}
	return nil
}
