package authflow

import (
	"context"

	"github.com/gotd/td/telegram/auth"
)

type FullAuthFlow interface {
	auth.UserAuthenticator

	GetAPICredentials(ctx context.Context) (int, string, error)
}
