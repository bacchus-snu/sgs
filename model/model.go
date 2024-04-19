package model

import (
	"crypto/des"
	"encoding/binary"
	"errors"
	"strings"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
)

const (
	charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	hashLen = 13
)

// good enough
var permut, _ = des.NewCipher([]byte("deadbeef"))

type ID int64

// Hash is a short, consistent hash for the ID.
func (id ID) Hash() string {
	b := binary.LittleEndian.AppendUint64(nil, uint64(id))
	permut.Encrypt(b, b)
	randID := binary.LittleEndian.Uint64(b)

	s := make([]byte, hashLen)
	for i := range s {
		s[i] = charset[randID%uint64(len(charset))]
		randID /= uint64(len(charset))
	}
	return string(s)
}

func ParseID(s string) (ID, error) {
	acc := int64(0)
	mul := int64(1)
	for _, c := range s {
		ind := int64(strings.IndexRune(charset, c))
		ind *= mul
		acc += ind
		mul *= int64(len(charset))
	}

	b := binary.LittleEndian.AppendUint64(nil, uint64(acc))
	permut.Decrypt(b, b)
	id := ID(binary.LittleEndian.Uint64(b))

	// sanity check
	if id < 0 || id.Hash() != s {
		return 0, ErrInvalid
	}

	return id, nil
}
