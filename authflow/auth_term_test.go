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
			"123\n+64221234567",
			strcls + phoneWelcome + phonePrompt + phoneMustIntl + "\n" + phonePrompt,
			"+64221234567",
			false,
		},
		{
			"phone is not set, not intl format",
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
			got, output, err := captureOutput(t, tt.input, a.Phone)
			if (err != nil) != tt.wantErr {
				t.Errorf("TermAuth.Phone() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantOut, output)
		})
	}
}

func captureOutput[T any](t *testing.T, input string, f func(ctx context.Context) (T, error)) (T, string, error) {
	t.Helper()
	// Save current stdout and stderr
	var (
		oldOut = hOutput
		oldIn  = hInput
	)

	// stdout pipe
	r, w, _ := os.Pipe()
	hOutput = w

	if len(input) > 0 {
		// stdin pipe
		r2, w2, _ := os.Pipe()
		hInput = r2
		go func() {
			lines := strings.Split(input, "\n")
			for _, line := range lines {
				n, err := w2.WriteString(line + "\n")
				if err != nil {
					t.Log(err)
				} else {
					t.Logf("captureOutput: wrote %d bytes\n", n)
				}
			}
			w2.WriteString("\n")
			w2.Close()
		}()
	}

	// Run function
	out, err := f(context.Background())

	// Read output
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = oldOut
	os.Stdin = oldIn

	return out, buf.String(), err
}
