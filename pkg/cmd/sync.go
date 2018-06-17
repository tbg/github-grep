package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tschottdorf/github-grep/pkg/searcher"
)

var rebuild bool

func init() {
	syncCmd.Flags().BoolVarP(&rebuild, "rebuild", "r", false, "Force rebuild from scratch")
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Update database of issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config()
		if err != nil {
			return err
		}
		cfg.Populate()
		defer cfg.Close()

		s := searcher.NewSearcher(cfg)
		return s.Sync(rebuild)
	},
}
