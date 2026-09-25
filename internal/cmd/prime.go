package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/prime"
)

var (
	prime5h               bool
	primeInclude5h        bool
	primeWarm5h           bool
	primeForce            bool
	primeCheck            bool
	primeStatus           bool
	primeNoJitter         bool
	primeMaxJitter        float64
	primeQuiet            bool
	primeInstallCron      bool
	primeUninstallCron    bool
	primeInstallSystemd   bool
	primeUninstallSystemd bool
	primeInstallTask      bool
	primeUninstallTask    bool
	primeJSON             bool
)

func newPrimeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "prime [profile]",
		Short: "Prime weekly token quota and manage reset automations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				prime5h = false
				primeInclude5h = false
				primeWarm5h = false
				primeForce = false
				primeCheck = false
				primeStatus = false
				primeNoJitter = false
				primeMaxJitter = 60.0
				primeQuiet = false
				primeJSON = false
				primeInstallCron = false
				primeUninstallCron = false
				primeInstallSystemd = false
				primeUninstallSystemd = false
				primeInstallTask = false
				primeUninstallTask = false
				_ = cmd.Flags().Set("json", "false")
				_ = cmd.Flags().Set("status", "false")
				_ = cmd.Flags().Set("check", "false")
				_ = cmd.Flags().Set("force", "false")
				_ = cmd.Flags().Set("no-jitter", "false")
				_ = cmd.Flags().Set("5h", "false")
				_ = cmd.Flags().Set("include-5h", "false")
				_ = cmd.Flags().Set("warm-5h", "false")
			}()

			profile := ""
			if len(args) > 0 {
				profile = args[0]
			}

			opts := prime.PrimeOptions{
				Profile:          profile,
				Force:            primeForce,
				Check:            primeCheck,
				Status:           primeStatus,
				NoJitter:         primeNoJitter,
				MaxJitter:        primeMaxJitter,
				Include5h:        prime5h || primeInclude5h,
				Warm5h:           primeWarm5h,
				Quiet:            primeQuiet,
				JSON:             primeJSON,
				Out:              cmd.OutOrStdout(),
				InstallCron:      primeInstallCron,
				UninstallCron:    primeUninstallCron,
				InstallSystemd:   primeInstallSystemd,
				UninstallSystemd: primeUninstallSystemd,
				InstallTask:      primeInstallTask,
				UninstallTask:    primeUninstallTask,
			}

			return prime.RunPrime(opts)
		},
	}

	c.Flags().BoolVar(&primeJSON, "json", false, "Output results in JSON format")

	c.Flags().BoolVar(&prime5h, "5h", false, "Include 5-hour quota reset windows (gemini-5h and 3p-5h)")
	c.Flags().BoolVar(&primeInclude5h, "include-5h", false, "Include 5-hour quota reset windows")
	c.Flags().BoolVar(&primeWarm5h, "warm-5h", false, "Trigger proactive warm-up ping for 5-hour rolling windows to start renewal clock early")
	c.Flags().BoolVarP(&primeForce, "force", "f", false, "Force prime regardless of current reset cycle or jitter schedule")
	c.Flags().BoolVar(&primeCheck, "check", false, "Check if quota is ready for priming without sending prompts")
	c.Flags().BoolVar(&primeStatus, "status", false, "Show prime automation and quota status")
	c.Flags().BoolVar(&primeNoJitter, "no-jitter", false, "Bypass random delay and prime immediately if ready")
	c.Flags().Float64Var(&primeMaxJitter, "jitter", 60.0, "Maximum random delay window in minutes")
	c.Flags().BoolVar(&primeQuiet, "quiet", false, "Suppress non-essential output (ideal for cron or headless jobs)")
	c.Flags().BoolVar(&primeInstallCron, "install-cron", false, "Install hourly crontab job for automatic priming")
	c.Flags().BoolVar(&primeUninstallCron, "uninstall-cron", false, "Remove hourly crontab job")
	c.Flags().BoolVar(&primeInstallSystemd, "install-systemd", false, "Install and activate hourly systemd user timer")
	c.Flags().BoolVar(&primeUninstallSystemd, "uninstall-systemd", false, "Remove and stop systemd user service and timer")
	c.Flags().BoolVar(&primeInstallTask, "install-task", false, "Install hourly Windows Scheduled Task")
	c.Flags().BoolVar(&primeUninstallTask, "uninstall-task", false, "Remove Windows Scheduled Task")

	return c
}

var primeCmd = newPrimeCmd()

func init() {
	rootCmd.AddCommand(primeCmd)
}
