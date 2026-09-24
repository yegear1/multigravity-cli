package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update multigravity to the latest version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := os.Executable()
		if err != nil {
			return fmt.Errorf("could not determine executable path for update: %w", err)
		}
		target, err = filepath.EvalSymlinks(target)
		if err != nil {
			return fmt.Errorf("could not resolve executable symlink: %w", err)
		}

		repo := os.Getenv("MULTIGRAVITY_REPO")
		if repo == "" {
			repo = "yegear1/multigravity-cli"
		}

		assetName := fmt.Sprintf("multigravity-%s-%s", runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			assetName += ".exe"
		}

		releaseURL := fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, assetName)

		cmd.Printf("Updating multigravity from %s ...\n", releaseURL)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Get(releaseURL)
		if err != nil {
			return fmt.Errorf("failed to download update: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download update from %s: HTTP %d %s", releaseURL, resp.StatusCode, resp.Status)
		}

		tmpFile := target + ".tmp"
		f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("could not create temporary file for update: %w", err)
		}

		_, err = io.Copy(f, resp.Body)
		f.Close()
		if err != nil {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("failed while saving update: %w", err)
		}

		// On Windows, running binary cannot be replaced while open, rename current to .old first
		if runtime.GOOS == "windows" {
			oldFile := target + ".old"
			_ = os.Remove(oldFile)
			if err := os.Rename(target, oldFile); err != nil {
				_ = os.Remove(tmpFile)
				return fmt.Errorf("could not move existing binary on Windows: %w", err)
			}
		}

		if err := os.Rename(tmpFile, target); err != nil {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("failed to replace binary: %w", err)
		}

		cmd.Println("Successfully updated multigravity!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
