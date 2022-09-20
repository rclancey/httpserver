package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"sync"
)

var (
	ErrClosed = errors.New("object stream already closed")
	ErrNotClosed = errors.New("object stream still open")
	ErrHeaderSent = errors.New("header already sent")
	ErrFooterSent = errors.New("footer already sent")
)

var (
	comma = []byte{','}
	colon = []byte{':'}
	openBrace = []byte{'{'}
	closeBrace = []byte{'}'}
	openBracket = []byte{':', '['}
	closeBracket = []byte{']'}
)

type ObjectStream struct {
	key string
	objects chan interface{}
	header map[string]interface{}
	headerKeys []string
	footer map[string]interface{}
	footerKeys []string
	mutex *sync.Mutex
	closed bool
}

func NewObjectStream(key string) *ObjectStream {
	return &ObjectStream{
		key: key,
		objects: make(chan interface{}, 1),
		header: map[string]interface{}{},
		footer: map[string]interface{}{},
		mutex: &sync.Mutex{},
		closed: false,
	}
}

// Send should be called from a goroutine after returning the ObjectStream
// from the handler
func (stream *ObjectStream) Send(obj interface{}) error {
	if stream.closed {
		return ErrClosed
	}
	stream.objects <- obj
	return nil
}

// SetHeader() should be called before returning the ObjectStream from the
// handler, rather than in a goroutine
func (stream *ObjectStream) SetHeader(key string, val interface{}) error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	if stream.header == nil {
		return ErrHeaderSent
	}
	stream.header[key] = val
	stream.headerKeys = append(stream.headerKeys, key)
	return nil
}

// SetFooter() must be called before Close()
func (stream *ObjectStream) SetFooter(key string, val interface{}) error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	if stream.closed {
		return ErrClosed
	}
	if stream.footer == nil {
		return ErrFooterSent
	}
	stream.footer[key] = val
	stream.footerKeys = append(stream.footerKeys, key)
	return nil
}

// Close() must be called at the end of the goroutine in order flush the
// stream to the client
func (stream *ObjectStream) Close() error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	if stream.closed {
		return ErrClosed
	}
	stream.closed = true
	close(stream.objects)
	return nil
}

func (stream *ObjectStream) writeKeyValue(w io.Writer, key string, val interface{}) error {
	data, err := json.Marshal(key)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	_, err = w.Write(colon)
	if err != nil {
		return err
	}
	data, err = json.Marshal(val)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (stream *ObjectStream) writeHeader(w io.Writer) error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	if stream.header == nil {
		return ErrHeaderSent
	}
	_, err := w.Write(openBrace)
	if err != nil {
		return err
	}
	for _, k := range stream.headerKeys {
		v, ok := stream.header[k]
		if !ok {
			continue
		}
		delete(stream.header, k)
		err = stream.writeKeyValue(w, k, v)
		if err != nil {
			return err
		}
		_, err = w.Write(comma)
		if err != nil {
			return err
		}
	}
	stream.header = nil
	stream.headerKeys = nil
	data, err := json.Marshal(stream.key)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	_, err = w.Write(openBracket)
	return err
}

func (stream *ObjectStream) writeFooter(w io.Writer) error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	if !stream.closed {
		return ErrNotClosed
	}
	if stream.footer == nil {
		return ErrFooterSent
	}
	_, err := w.Write(closeBracket)
	if err != nil {
		return err
	}
	for _, k := range stream.footerKeys {
		v, ok := stream.footer[k]
		if !ok {
			continue
		}
		delete(stream.footer, k)
		_, err = w.Write(comma)
		if err != nil {
			return err
		}
		err = stream.writeKeyValue(w, k, v)
		if err != nil {
			return err
		}
	}
	stream.footer = nil
	stream.footerKeys = nil
	_, err = w.Write(closeBrace)
	return err
}

func (stream *ObjectStream) stream(w io.Writer) error {
	err := stream.writeHeader(w)
	if err != nil {
		return err
	}
	first := true
	for {
		obj, ok := <-stream.objects
		if !ok {
			break
		}
		if first {
			first = false
		} else {
			_, err = w.Write(comma)
			if err != nil {
				return err
			}
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		if err != nil {
			return err
		}
	}
	return stream.writeFooter(w)
}
