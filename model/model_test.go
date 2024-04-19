package model

import (
	"errors"
	"math"
	"testing"
)

func TestHashConsistent(t *testing.T) {
	known := map[ID]string{
		42:            "eveajpbf7nxa3",
		math.MaxInt64: "tmqpptmoi0ky2",
	}

	for id, want := range known {
		hash := id.Hash()
		if hash != want {
			t.Errorf("Hash(%d) = %s; want %s", id, hash, want)
		}

		rev, err := ParseID(hash)
		if err != nil {
			t.Errorf("ParseID(%s) = %v; want nil", hash, err)
		} else if rev != id {
			t.Errorf("ParseID(%s) = %d; want %d", hash, rev, id)
		}
	}
}

func TestParseID(t *testing.T) {
	_, err := ParseID("invalid")
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("ParseID(invalid) = %v; want %v", err, ErrInvalid)
	}
}

func TestHashlen(t *testing.T) {
	want := int(math.Ceil(64 / math.Log2(float64(len(charset)))))
	if want != hashLen {
		t.Errorf("expected %d, got %d", want, hashLen)
	}
}
