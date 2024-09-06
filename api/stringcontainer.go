package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/tysonmote/gommap"
	"math"
	"os"
)

type StringContainer struct {
	file          *os.File
	mmap          gommap.MMap
	currentOffset uint32
}

func NewStringContainer(filename string) (*StringContainer, error) {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	mmap, err := gommap.Map(f.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	return &StringContainer{
		file: f,
		mmap: mmap,
	}, nil
}

func (sc *StringContainer) Add(value string) uint32 {
	startOffset := sc.currentOffset
	byteOffset := startOffset
	length := len([]byte(value))
	if length > math.MaxUint16 {
		log.Warn().Msgf("string too long %d: %s", length, value)
	}
	if byteOffset+uint32(length) >= math.MaxUint32 {
		panic(fmt.Sprintf("mmap out of space %d: %s", byteOffset+uint32(length), value))
	}
	for _, b := range []byte(value) {
		sc.mmap[byteOffset] = b
		byteOffset++
	}
	sc.mmap[byteOffset] = 0
	byteOffset++
	sc.currentOffset = byteOffset
	return startOffset
}

func (sc *StringContainer) Get(offset uint32) string {
	byteOffset := offset
	var stringReading []byte
	for i := 0; i < 1024*16; i++ {
		if sc.mmap[byteOffset] == 0 {
			break
		}
		stringReading = append(stringReading, sc.mmap[byteOffset])
		byteOffset++
	}
	result := string(stringReading)
	return result
}

func (sc *StringContainer) Flush() {
	sc.mmap.Sync(gommap.MS_SYNC)
}
