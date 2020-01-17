/*
 * LRU for little file Read
 */

package lru

import (
	"os"
	"fmt"
	"sync"
	"strconv"
)

type LRUProvider interface {
	Read(filename string) ([]byte, error)
	debug()
}

type lruProvider struct {
	Cache map[string]*element
	CacheSize int64
	MaxSize int64
	Header *element
	Tailer *element
	sync.Mutex
}

type element struct {
	name		string
	bufSize		int64
	content		[]byte
	prevElem	*element
	postElem	*element
}

func NewLRUProvider(size int64) LRUProvider {
	return &lruProvider{
		Cache: make(map[string]*element),
		CacheSize:0,
		MaxSize:size,
		Header:nil,
		Tailer:nil,
	}
}

func (lru *lruProvider) Read(filename string) ([]byte, error) {

	if item, ok := lru.Cache[filename]; ok {
		lru.Lock()
		defer lru.Unlock()
		if lru.Header != item {
			curr := item
			prev := curr.prevElem
			post := curr.postElem
			if lru.Tailer == item {
				prev.postElem = nil
				lru.Tailer = prev
			} else {
				prev.postElem = post
				post.prevElem = prev
			}
			curr.prevElem = nil
			curr.postElem = lru.Header
			lru.Header.prevElem = curr
			lru.Header = curr
		}
		return item.content, nil
	} else {
		if fp, err := os.Open(filename); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot open file:%s(err=%v)\n", filename, err)
			return nil, err
		} else {
			defer fp.Close()
			if fs, err := fp.Stat(); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot get file info:%s(err=%v)\n", filename, err)
				return nil, err
			} else {
				fsize := fs.Size()
				content := make([]byte, fsize)
				if _, err := fp.Read(content); err != nil {
					fmt.Fprintf(os.Stderr, "Cannot read file:%s(err=%v)\n", filename, err)
					return nil, err
				}
				
				lru.Lock()
				defer lru.Unlock()
				if fsize > lru.MaxSize { // No Cache
					return content, nil
				}
				// Remove files for Tailer
				if fsize > lru.MaxSize - lru.CacheSize {
					// Remove from tailer
					availSize := lru.MaxSize - lru.CacheSize
					for fsize >= availSize {
						pointer := lru.Tailer
						if pointer == lru.Header {
							lru.Header = nil
							lru.Tailer = nil
						} else {
							prev := pointer.prevElem
							prev.postElem = nil
							lru.Tailer = prev
						}
						lru.CacheSize -= pointer.bufSize
						delete(lru.Cache, pointer.name)
						
						pointer.prevElem = nil
						pointer.postElem = nil

						availSize += pointer.bufSize
					}
				}
				// Add file 
				header := lru.Header
				elem := &element{
					name: filename,
					bufSize: fsize,
					content: content,
					prevElem: nil,
					postElem: header,
				}
				lru.Header = elem
				if header == nil {
					lru.Tailer = elem
				} else {
					header.prevElem = elem
				}
				lru.CacheSize += fsize
				lru.Cache[filename] = elem
				
				return content, nil
			}
		}
		
	}
}

func (lru *lruProvider) debug() {
	fmt.Fprintf(os.Stdout, "LRU : MaxSize:%d, CacheSize:%d\n", lru.MaxSize, lru.CacheSize)
	fmt.Fprintf(os.Stdout, "LRU : Header:%p, Tailer:%p\n", lru.Header, lru.Tailer)
	fmt.Fprintf(os.Stdout, "LRU : Map(%d):\n", len(lru.Cache))
	for k,v := range lru.Cache {
		fmt.Fprintf(os.Stdout, "LRU : Key(%s), Value(%p)\n", k, v)
	}

	var item *element
	for item = lru.Header; item != lru.Tailer; item = item.postElem {
		fmt.Fprintf(os.Stdout, "        Elem : name:%s, bufSize:%d\n", item.name, item.bufSize)
		fmt.Fprintf(os.Stdout, "        Elem : prev:%p, post:%p\n", item.prevElem, item.postElem)
	}
	if item != nil {
		fmt.Fprintf(os.Stdout, "        Elem Tailer : name:%s, bufSize:%d\n", item.name, item.bufSize)
		fmt.Fprintf(os.Stdout, "        Elem Tailer : prev:%p, post:%p\n", item.prevElem, item.postElem)
	}
}
