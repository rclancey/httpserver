package httpserver

import (
	"bytes"
	"time"

	. "gopkg.in/check.v1"
)

type ObjectStreamSuite struct {}

var _ = Suite(&ObjectStreamSuite{})

type TestStreamObj struct {
	N int `json:"n"`
	T int64 `json:"t"`
}

func (s *ObjectStreamSuite) TestStream(c *C) {
	stream := NewObjectStream("test")
	stream.SetHeader("foo", "bar")
	stream.SetHeader("baz", []string{"a", "b"})
	go func() {
		stream.SetFooter("status", "unknown")
		start := time.Now().UnixMilli()
		ticker := time.NewTicker(10 * time.Millisecond)
		for i := 0; i < 10; i++ {
			t := <-ticker.C
			stream.Send(&TestStreamObj{N: i, T: (t.UnixMilli() - start)  / 10})
		}
		ticker.Stop()
		stream.SetFooter("count", 10)
		stream.SetFooter("status", "ok")
		stream.Close()
	}()
	buf := bytes.NewBuffer(nil)
	err := stream.stream(buf)
	c.Check(err, IsNil)
	exp := `{"foo":"bar","baz":["a","b"],"test":[{"n":0,"t":1},{"n":1,"t":2},{"n":2,"t":3},{"n":3,"t":4},{"n":4,"t":5},{"n":5,"t":6},{"n":6,"t":7},{"n":7,"t":8},{"n":8,"t":9},{"n":9,"t":10}],"status":"ok","count":10}`
	c.Check(string(buf.Bytes()), Equals, exp)
}
