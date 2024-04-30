package mtpwrap

import (
	"encoding/json"
	"io"

	"github.com/rusq/encio"
)

type credsStorage struct {
	filename string
}

// creds is the structure of data in the storage.
type creds struct {
	ID   int    `json:"api_id,omitempty"`
	Hash string `json:"api_hash,omitempty"`
}

func (c creds) IsEmpty() bool {
	return c.ID == 0 || c.Hash == ""
}

// IsAvailable returns true if the credentials filename is set.
func (cs credsStorage) IsAvailable() bool {
	return cs.filename != ""
}

func (cs credsStorage) Save(c creds) error {
	f, err := encio.Create(cs.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return cs.write(f, c)
}

func (cs credsStorage) write(f io.Writer, c creds) error {
	enc := json.NewEncoder(f)
	if err := enc.Encode(c); err != nil {
		return err
	}
	return nil
}

func (cs credsStorage) Load() (creds, error) {
	f, err := encio.Open(cs.filename)
	if err != nil {
		return creds{}, err
	}
	defer f.Close()

	return cs.read(f)
}

func (cs credsStorage) read(r io.Reader) (creds, error) {
	var cr creds
	dec := json.NewDecoder(r)
	if err := dec.Decode(&cr); err != nil {
		return creds{}, err
	}
	return cr, nil
}
