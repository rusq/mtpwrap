// Package mtpwrap provides some functions for the gotd/td functions
package mtpwrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/bluele/gcache"
	"github.com/gotd/contrib/bg"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/session"
	"github.com/gotd/td/tdp"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/mattn/go-colorable"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/rusq/mtpwrap/authflow"
)

const (
	defBatchSize  = 100
	defCacheEvict = 10 * time.Minute
	defCacheSz    = 20
)

var (
	// ErrAlreadyRunning is returned if the attempt is made to start the client,
	// while there's another instance running asynchronously.
	ErrAlreadyRunning = errors.New("already running asynchronously, stop the running instance first")
)

// ErrAuth is returned if the authentication fails.
type ErrAuth struct {
	Err error
}

type Client struct {
	cl *telegram.Client

	cache     gcache.Cache
	peerStrg  storage.PeerStorage
	credsStrg credsStorage
	creds     creds // API credentials

	waiter     *floodwait.SimpleWaiter
	waiterStop func()

	stop bg.StopFunc

	auth         authflow.FullAuthFlow
	sendcodeOpts auth.SendCodeOptions
	telegramOpts telegram.Options
}

// Entity interface is the subset of functions that are commonly defined on most
// entities in telegram lib. It can be a user, a chat or channel, or any other
// telegram Entity.
type Entity interface {
	GetID() int64
	GetTitle() string
	TypeInfo() tdp.Type
	Zero() bool
}

type cacheKey int64

const (
	cacheDlgStorage cacheKey = iota
)

type Option func(c *Client)

func WithMTPOptions(opts telegram.Options) Option {
	return func(c *Client) {
		c.telegramOpts = opts
	}
}

// WithStorage allows to specify custom session storage.
func WithStorage(path string) Option {
	return func(c *Client) {
		c.telegramOpts.SessionStorage = &session.FileStorage{Path: path}
	}
}

// WithPeerStorage allows to specify a custom storage for peer data.
func WithPeerStorage(s storage.PeerStorage) Option {
	return func(c *Client) {
		if s == nil {
			return
		}
		c.peerStrg = s
	}
}

// WithAuth allows to override the authorization flow
func WithAuth(flow authflow.FullAuthFlow) Option {
	return func(c *Client) {
		c.auth = flow
	}
}

func WithApiCredsFile(path string) Option {
	return func(c *Client) {
		c.credsStrg = credsStorage{filename: path}
	}
}

func WithDebug(enable bool) Option {
	return func(c *Client) {
		if !enable {
			c.telegramOpts.Logger = nil
			return
		}
		cfg := zap.NewDevelopmentEncoderConfig()
		cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		c.telegramOpts.Logger = zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(cfg),
			zapcore.AddSync(colorable.NewColorableStdout()),
			zapcore.DebugLevel,
		))
	}
}

func New(ctx context.Context, appID int, appHash string, opts ...Option) (*Client, error) {
	// Client with the default parameters
	var c = Client{
		cache:    gcache.New(defCacheSz).LFU().Expiration(defCacheEvict).Build(),
		peerStrg: NewMemStorage(),

		auth:   authflow.TermAuth{}, // default is the terminal authentication
		waiter: floodwait.NewSimpleWaiter(),

		telegramOpts: telegram.Options{},
	}

	for _, opt := range opts {
		opt(&c)
	}

	var creds = creds{
		ID:   appID,
		Hash: appHash,
	}

	c.telegramOpts.Middlewares = append(c.telegramOpts.Middlewares, c.waiter)
	if creds.IsEmpty() && c.credsStrg.IsAvailable() {
		var err error
		creds, err = c.loadCredentials(ctx)
		if err != nil {
			return nil, err
		}
	}

	c.cl = telegram.NewClient(creds.ID, creds.Hash, c.telegramOpts)

	return &c, nil
}

func (c *Client) loadCredentials(ctx context.Context) (creds, error) {
	var err error
	creds, err := c.credsStrg.Load()
	if err == nil && !creds.IsEmpty() {
		return creds, nil
	}
	Log.Debugf("warning: error loading credentials file, requesting manual input: %s", err)
	creds.ID, creds.Hash, err = c.auth.GetAPICredentials(ctx)
	if err != nil {
		fmt.Println()
		if errors.Is(io.EOF, err) {
			return creds, errors.New("exit")
		}
		return creds, err
	}
	if creds.IsEmpty() {
		return creds, errors.New("invalid credentials")
	}
	return creds, nil
}

// Start starts the telegram session in goroutine
func (c *Client) Start(ctx context.Context) error {
	if c.stop != nil {
		return ErrAlreadyRunning
	}

	stop, err := bg.Connect(c.cl)
	if err != nil {
		return err
	}
	c.stop = stop

	flow := auth.NewFlow(c.auth, c.sendcodeOpts)
	if err := c.cl.Auth().IfNecessary(ctx, flow); err != nil {
		if err := c.Stop(); err != nil {
			Log.Debugf("error stopping: %s", err)
		}
		return &ErrAuth{Err: err}
	}
	Log.Debug("auth success")

	// try and save credentials now that we're sure they're correct.
	if err := c.saveCreds(); err != nil {
		// not a fatal error
		Log.Printf("failed to save credentials: %s, but nevermind let's continue", err)
	}

	return nil
}

func (c *Client) saveCreds() error {
	return c.credsStrg.Save(c.creds)
}

func (e *ErrAuth) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.Err)
}

func (e *ErrAuth) Unwrap() error {
	return e.Err
}

func (e *ErrAuth) Is(err error) bool {
	return errors.Is(e.Err, err)
}

func (c *Client) Stop() error {
	if c.stop != nil {
		if c.waiterStop != nil {
			defer c.waiterStop()
		}
		return c.stop()
	}
	return nil
}

// Run runs an arbitrary telegram session.
func (c *Client) Run(ctx context.Context, fn func(context.Context, *telegram.Client) error) error {
	if c.stop != nil {
		return ErrAlreadyRunning
	}
	return c.cl.Run(ctx, func(ctx context.Context) error {
		return fn(ctx, c.cl)
	})
}

// Client returns the underlying telegram client.
func (c *Client) Client() *telegram.Client {
	return c.cl
}
