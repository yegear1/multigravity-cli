package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/auth"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var (
	loginJSON      bool
	loginNoBrowser bool
	loginForce     bool
	loginTimeout   time.Duration
)

func newLoginCmd() *cobra.Command {
	loginCmd := &cobra.Command{
		Use:   "login <profile>",
		Short: "Sign in a profile with Google OAuth2 PKCE",
		Long: `Authenticate a profile through Google OAuth2 with PKCE and an ephemeral
loopback callback. The refresh token is written only to that profile's vault
and is never stored in the host keyring.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: profileArgsCompletion,
		RunE:              runLogin,
	}
	loginCmd.PersistentFlags().BoolVar(&loginJSON, "json", false, "Output results in machine-readable JSON format")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Print the authorization URL instead of opening a browser")
	loginCmd.Flags().BoolVar(&loginForce, "force", false, "Sign in even if the profile is currently running")
	loginCmd.Flags().DurationVar(&loginTimeout, "timeout", 3*time.Minute, "How long to wait for the browser callback")

	statusSub := &cobra.Command{
		Use:               "status <profile>",
		Short:             "Show whether a profile has a vault credential",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: profileArgsCompletion,
		RunE:              runLoginStatus,
	}
	logoutSub := &cobra.Command{
		Use:               "logout <profile>",
		Short:             "Delete the credential stored in a profile vault",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: profileArgsCompletion,
		RunE:              runLoginLogout,
	}
	logoutSub.Flags().BoolVar(&loginForce, "force", false, "Delete the vault credential even if the profile is running")

	loginCmd.AddCommand(statusSub, logoutSub)
	return loginCmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	defer resetLoginFlags(cmd)
	name := args[0]
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}
	if !profile.ProfileExists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	if !loginForce && profile.IsProfileRunning(name) {
		return fmt.Errorf("profile %q is running — stop it first or pass --force", name)
	}

	session, err := auth.Login(cmd.Context(), name, config.GetProfileDir(name), loginOpts(cmd))
	if err != nil {
		return err
	}
	if loginJSON {
		return writeLoginJSON(cmd, session)
	}
	who := session.Email
	if who == "" {
		who = "the authorized account"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Authenticated profile %q as %s\nCredential stored in the profile vault.\n", name, who)
	return nil
}

func runLoginStatus(cmd *cobra.Command, args []string) error {
	defer resetLoginFlags(cmd)
	name := args[0]
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}
	if !profile.ProfileExists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	session, err := auth.Status(name, config.GetProfileDir(name))
	if err != nil {
		return err
	}
	if loginJSON {
		return writeLoginJSON(cmd, session)
	}
	if !session.Authenticated {
		fmt.Fprintf(cmd.OutOrStdout(), "Profile %q is not authenticated\n", name)
		return nil
	}
	who := session.Email
	if who == "" {
		who = "unknown account"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Profile %q is authenticated as %s\n", name, who)
	return nil
}

func runLoginLogout(cmd *cobra.Command, args []string) error {
	defer resetLoginFlags(cmd)
	name := args[0]
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}
	if !profile.ProfileExists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	if !loginForce && profile.IsProfileRunning(name) {
		return fmt.Errorf("profile %q is running — stop it first or pass --force", name)
	}
	if err := auth.Logout(config.GetProfileDir(name)); err != nil {
		return err
	}
	if loginJSON {
		return writeLoginJSON(cmd, auth.Session{Profile: name, Authenticated: false})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed the credential from profile %q\n", name)
	return nil
}

func resetLoginFlags(cmd *cobra.Command) {
	loginJSON = false
	loginNoBrowser = false
	loginForce = false
	loginTimeout = 3 * time.Minute
	target := cmd
	if cmd.Parent() != nil && cmd.Name() != "login" {
		target = cmd.Parent()
	}
	_ = target.PersistentFlags().Set("json", "false")
	if f := cmd.Flags().Lookup("no-browser"); f != nil {
		_ = f.Value.Set("false")
	}
	if f := cmd.Flags().Lookup("force"); f != nil {
		_ = f.Value.Set("false")
	}
	if f := cmd.Flags().Lookup("timeout"); f != nil {
		_ = f.Value.Set("3m0s")
	}
}

func loginOpts(cmd *cobra.Command) auth.Options {
	opts := auth.Options{
		NoBrowser: loginNoBrowser,
		Timeout:   loginTimeout,
		NotifyURL: func(authURL string) {
			fmt.Fprintln(cmd.ErrOrStderr(), "Open this URL to sign in:")
			fmt.Fprintln(cmd.ErrOrStderr(), authURL)
		},
	}
	if prepareLoginOptions != nil {
		prepareLoginOptions(&opts)
	}
	return opts
}

// prepareLoginOptions is set by tests to point login at a local token server.
var prepareLoginOptions func(*auth.Options)

func writeLoginJSON(cmd *cobra.Command, session auth.Session) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(session)
}
