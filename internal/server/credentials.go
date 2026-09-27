package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/auth"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/gateway"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func resolveProfileAccessToken(r *http.Request, profileName string) (string, error) {
	if profileName == "" || profileName == "default" {
		return "", fmt.Errorf("profile %q is not authenticated", profileName)
	}
	if err := config.ValidateProfileName(profileName); err != nil {
		return "", err
	}
	token, err := auth.AccessToken(r.Context(), config.GetProfileDir(profileName), auth.Options{})
	if errors.Is(err, auth.ErrNotAuthenticated) {
		return "", fmt.Errorf("profile %q is not authenticated", profileName)
	}
	return token, err
}

func refreshProfileQuota(_ context.Context, router *gateway.Router) {
	if router == nil {
		return
	}
	servers, err := quota.FindActiveServers("")
	if err != nil {
		return
	}
	now := time.Now()
	for _, srv := range servers {
		if srv.Profile == "" || srv.Profile == "host" || srv.Data == nil {
			continue
		}
		fraction, reset, ok := quota.RoutingFraction(srv.Data, now)
		if !ok {
			continue
		}
		router.UpdateQuota(srv.Profile, fraction, reset)
	}
}
