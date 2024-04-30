package mtpwrap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_encryptDecrypt(t *testing.T) {
	var (
		ApiID   = 12345
		ApiHash = "very secure"
	)
	var buf bytes.Buffer
	cs := credsStorage{}
	err := cs.write(&buf, creds{ApiID, ApiHash})
	assert.NoError(t, err)

	got, gotErr := cs.read(&buf)
	assert.NoError(t, gotErr)
	assert.Equal(t, ApiID, got.ID)
	assert.Equal(t, ApiHash, got.Hash)

}

func FuzzWriteRead(f *testing.F) {
	type testcase struct {
		id   int
		hash string
	}
	var testcases = []testcase{{12345, "very secure"}, {0, "12345"}, {42, ""}, {-100, "blah"}}
	for _, tc := range testcases {
		f.Add(tc.id, tc.hash)
	}
	cs := credsStorage{}
	f.Fuzz(func(t *testing.T, id int, hash string) {
		var buf bytes.Buffer
		err := cs.write(&buf, creds{id, hash})
		if err != nil {
			return
		}
		got, gotErr := cs.read(&buf)
		if gotErr != nil {
			return
		}
		assert.Equal(t, id, got.ID)
		assert.Equal(t, hash, got.Hash)
	})
}
