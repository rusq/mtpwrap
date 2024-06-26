package authflow

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/term"
)

var (
	blink     = color.New(color.BlinkSlow)
	italic    = color.New(color.Italic)
	param     = color.New(color.Italic, color.FgBlue, color.BgHiWhite)
	warn      = color.New(color.FgHiRed)
	underline = color.New(color.Underline)

	line = strings.Repeat("-=", 40)
)

// noSignUp can be embedded to prevent signing up.
type noSignUp struct{}

func (c noSignUp) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("not implemented")
}

func (c noSignUp) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

// TermAuth implements authentication via terminal.
type TermAuth struct {
	noSignUp

	phone string
}

func NewTermAuth(phone string) TermAuth {
	return TermAuth{phone: phone}
}

var (
	hOutput = os.Stdout
	hInput  = os.Stdin
)

const (
	phoneWelcome = `Connected, please login to Telegram.

Enter phone in international format, no spaces, for example +6422123456
`
	phonePrompt     = "PHONE> "
	phoneInvalid    = "*** Invalid phone number, try again or press Ctrl+C to abort ***"
	phoneMustIntl   = "*** Phone number must be in international format, starting with '+' ***"
	phoneOnlyDigits = "*** Phone number must contain only digits and + sign. ***"
)

func (a TermAuth) Phone(_ context.Context) (string, error) {
	clrscr(hOutput)
	if a.phone != "" {
		return a.phone, nil
	}
	fmt.Fprint(hOutput, phoneWelcome)
	for {
		fmt.Fprint(hOutput, phonePrompt)
		phone, err := readln(hInput)
		if err != nil {
			return "", err
		}
		if phone == "" {
			fmt.Fprintln(hOutput, phoneInvalid)
			continue
		}
		if !strings.HasPrefix(phone, "+") || len(phone) < 2 {
			fmt.Fprintln(hOutput, phoneMustIntl)
			continue
		}
		if _, err := strconv.Atoi(phone[1:]); err != nil {
			fmt.Fprintln(hOutput, phoneOnlyDigits)
			continue
		}
		return phone, nil
	}
}

func (a TermAuth) Password(ctx context.Context) (string, error) {
	defer fmt.Fprintln(hOutput)
	fmt.Fprintln(hOutput, "Enter 2FA password (won't be shown)")
	fmt.Fprint(hOutput, "2FA> ")
	return readpass(ctx, hInput)
}

func codeSpecifics(code *tg.AuthSentCode) (string, int) {
	digits := func(where string, n int) string {
		return fmt.Sprintf("The code %s.\nEnter exactly %d digits.", where, n)
	}

	switch val := code.Type.(type) {
	case *tg.AuthSentCodeTypeApp:
		return digits("was sent through the telegram app", val.GetLength()), val.GetLength()
	case *tg.AuthSentCodeTypeSMS:
		return digits("will be sent via a text message (SMS)", val.GetLength()), val.GetLength()
	case *tg.AuthSentCodeTypeCall:
		return digits("will be sent via a phone call, and a synthesized voice will tell you what to input", val.GetLength()), val.GetLength()
	case *tg.AuthSentCodeTypeFlashCall:
		return fmt.Sprintf("The code will be sent via a flash phone call, that will be closed immediately.\nThe phone code will then be the phone number itself, just make sure that the\nphone number matches the specified pattern: %q (%d characters)", val.GetPattern(), len(val.GetPattern())), len(val.GetPattern())
	case *tg.AuthSentCodeTypeMissedCall:
		return fmt.Sprintf("The code will be sent via a flash phone call, that will be closed immediately.\nThe last digits of the phone number that calls are the code that must be entered.\nThe phone call prefix will be: %s and the length of the code is %d", val.GetPrefix(), val.GetLength()), val.GetLength()
	default:
		return "UNSUPPORTED AUTH TYPE", 0
	}
}

func codeTimeout(code *tg.AuthSentCode) (string, time.Duration) {
	timeout, ok := code.GetTimeout()
	if !ok {
		return "", 30 * time.Minute
	}
	ret := time.Duration(timeout) * time.Second
	return fmt.Sprintf("(enter code within %s)", ret), ret
}

func (a TermAuth) Code(_ context.Context, code *tg.AuthSentCode) (string, error) {
	codeHelp, length := codeSpecifics(code)
	timeoutHelp, timeoutIn := codeTimeout(code)
	timeout := time.Now().Add(timeoutIn)

	var input string
	var err error
	for {
		if time.Now().After(timeout) {
			return "", errors.New("operation timed out")
		}
		fmt.Printf("(i) TIP: %s\nCODE%s> ", codeHelp, timeoutHelp)
		input, err = readln(hInput)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errors.New("login aborted")
			}
		}
		if len(input) == length || length == 0 {
			break
		}
		fmt.Println("*** Invalid code, try again [Press Ctrl+C to abort] ***")
	}
	return input, nil
}

func (a TermAuth) GetAPICredentials(ctx context.Context) (int, string, error) {
	instructions()
	var id int
	for {
		fmt.Printf("Enter App '%s': ", param.Sprint(" api_id "))
		sID, err := readln(hInput)
		if err != nil {
			return 0, "", err
		}
		id, err = strconv.Atoi(sID)
		if err == nil {
			break
		}
		fmt.Println("*** Input error: api_id should be an integer")
	}
	fmt.Printf("Enter App '%s' (won't be shown): ", param.Sprint(" api_hash "))
	hash, err := readpass(ctx, hInput)
	fmt.Println()
	if err != nil {
		return 0, "", err
	}
	return id, hash, nil
}

func instructions() {

	fmt.Println(line)
	fmt.Printf("To get the API ID and API Hash, follow the instructions:\n\n")
	fmt.Printf("\t1.  Login to telegram \"API Development tools\":\n")
	fmt.Printf("\t\t%s %s %s\n", blink.Sprint("->"), italic.Sprint("https://my.telegram.org/apps"), blink.Sprint("<-"))
	fmt.Printf("\t2.  Fill in the form:  %s, %s and %s can be any values\n\t    you like;\n"+
		"\t3.  Choose \"%s\" platform\n"+
		"\t4.  Click <Create Application> button.\n\n",
		underline.Sprint("App title"), underline.Sprint("Short Name"), underline.Sprint("URL"),
		underline.Sprint("Desktop"))
	fmt.Printf("You will see the App '%s' and App '%s' values that you will need to\n"+
		"enter shortly.  This application will encrypt and save the credentials on your\ndevice.  You can delete them any time starting with -reset flag.\n\n",
		param.Sprint(" api_id "), param.Sprint(" api_hash "))
	warn.Printf("VERY IMPORTANT: This is the key to your account, keep it secret, never share\n" +
		"it with anyone, never publish it online.\n")
	fmt.Println(line)
	fmt.Println()
}

var readln = func(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readpass(_ context.Context, r *os.File) (string, error) {
	fd := int(r.Fd())

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, oldState)

	bytePwd, err := term.ReadPassword(fd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytePwd)), nil
}
