package authflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var strcls string

func init() {
	// get the OS specific clear screen
	var buf bytes.Buffer
	clrscr(&buf)
	strcls = buf.String()
}

func TestTermAuth_Phone(t *testing.T) {
	type fields struct {
		noSignUp noSignUp
		phone    string
	}
	type args struct {
		in0 context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		input   string
		wantOut string
		want    string
		wantErr bool
	}{
		{
			"phone is already set",
			fields{phone: "123"},
			args{context.Background()},
			"",
			strcls,
			"123",
			false,
		},
		{
			"phone is not set",
			fields{phone: ""},
			args{context.Background()},
			"+64221234567",
			strcls + phoneWelcome + phonePrompt,
			"+64221234567",
			false,
		},
		{
			"phone is not set, invalid input",
			fields{phone: ""},
			args{context.Background()},
			"\n+64221234567",
			strcls + phoneWelcome + phonePrompt + phoneInvalid + "\n" + phonePrompt,
			"+64221234567",
			false,
		},
		{
			"phone is not set, not intl format",
			fields{phone: ""},
			args{context.Background()},
			"123\n+64221234567\n",
			strcls + phoneWelcome + phonePrompt + phoneMustIntl + "\n" + phonePrompt,
			"+64221234567",
			false,
		},
		{
			"phone is not set, non-digit chars",
			fields{phone: ""},
			args{context.Background()},
			"+64 22 123 45 67\n+64221234567",
			strcls + phoneWelcome + phonePrompt + phoneOnlyDigits + "\n" + phonePrompt,
			"+64221234567",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := TermAuth{
				noSignUp: tt.fields.noSignUp,
				phone:    tt.fields.phone,
			}

			cap := StartCapture(t, strings.Split(tt.input, "\n")...)
			got, err := a.Phone(tt.args.in0)
			output := cap.StopCapture()

			if (err != nil) != tt.wantErr {
				t.Errorf("TermAuth.Phone() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantOut, output)
		})
	}
}

type captor struct {
	r         *os.File
	w         *os.File
	oldOut    *os.File
	oldReadln func(r io.Reader) (string, error)
}

// StartCapture starts capturing output. If input is not empty, it will also
// capture input and write it to the program's stdin.
func StartCapture(t *testing.T, input ...string) *captor {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut := hOutput
	oldIn := readln

	hOutput = w

	if len(input) > 0 {
		readln = mkTestReadln(input...)
	}

	return &captor{
		r:         r,
		w:         w,
		oldOut:    oldOut,
		oldReadln: oldIn,
	}
}

func mkTestReadln(input ...string) func(io.Reader) (string, error) {
	if len(input) == 0 {
		return func(r io.Reader) (string, error) { return "", nil }
	}
	var i = 0
	return func(r io.Reader) (string, error) {
		if i >= len(input) {
			return "", io.EOF
		}
		ret := input[i]
		i++
		return ret, nil
	}
}

// StopCapture stops capturing and returns the captured output
func (c *captor) StopCapture() string {
	c.w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, c.r)
	os.Stdout = c.oldOut
	readln = c.oldReadln

	return buf.String()
}

func Test_readln(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			"empty input",
			"\n",
			"",
			false,
		},
		{
			"valid input",
			"123\n",
			"123",
			false,
		},
		{
			"valid input with spaces",
			"  123  \n",
			"123",
			false,
		},
		{
			"multiple lines (reads only first line)",
			"123\n456\n",
			"123",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readln(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("readln() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("readln() = %v, want %v", got, tt.want)
			}
		})
	}
}
