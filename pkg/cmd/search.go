package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/marcusolsson/tui-go"
	"github.com/spf13/cobra"
	"github.com/tschottdorf/github-grep/pkg/searcher"
	"golang.org/x/crypto/ssh/terminal"
)

var quiet bool

func init() {
	searchCmd.Flags().BoolVarP(
		&quiet, "quiet", "q",
		!terminal.IsTerminal(int(os.Stdout.Fd())),
		"Print issue numbers only",
	)
	rootCmd.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search issues and comments",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config()
		if err != nil {
			return err
		}
		cfg.Populate()
		defer cfg.Close()

		if cfg.AccessToken == "" {
			glog.Warningf("please set the %s environment variable to a Github access token",
				tokenEnvVar,
			)
		}

		s := searcher.NewSearcher(cfg)

		if quiet {
			results, err := s.Search(strings.Join(args, " "), 100)
			if err != nil {
				return err
			}
			for _, result := range results {
				fmt.Println(result.IssueNumber)
			}
			return nil
		}

		if n, err := s.Num(); err != nil {
			return err
		} else if n == 0 {
			if err := syncCmd.RunE(cmd, nil); err != nil {
				return err
			}
		}

		table := tui.NewTable(0, 0)
		table.SetColumnStretch(0, 1)
		table.SetColumnStretch(1, 4)
		table.SetColumnStretch(2, 6)
		table.SetSizePolicy(tui.Minimum, tui.Maximum)

		var results []searcher.Result    // ui goroutine only
		updateSearch := func(q string) { // ui goroutine only
			var err error
			// NB: when I drop the limit here the app sometimes gets stuck
			// in what looks like an infinite repaint loop. Not sure what's
			// up with that; maybe there are too many results and it hits
			// some kind of inefficiency.
			results, err = s.Search(q, 100)
			table.RemoveRows()
			if err != nil {
				table.AppendRow(tui.NewLabel("error: " + err.Error()))
			}

			for _, result := range results {
				table.AppendRow(
					tui.NewLabel(strconv.Itoa(result.IssueNumber)),
					tui.NewHBox(tui.NewLabel(result.Title), tui.NewSpacer()),
					tui.NewLabel(strings.Replace(result.Excerpt, "\n", " ", -1)),
				)
			}
		}

		input := tui.NewEntry()
		input.SetFocused(true)
		input.SetSizePolicy(tui.Expanding, tui.Maximum)

		inputBox := tui.NewHBox(input)
		inputBox.SetBorder(true)
		inputBox.SetSizePolicy(tui.Expanding, tui.Maximum)

		root := tui.NewVBox(table, tui.NewSpacer(), inputBox)
		ui := tui.New(root)

		teardown := make(chan struct{})
		defer close(teardown)

		searches := make(chan string, 10)
		go func() {
			closedCh := make(chan struct{})
			close(closedCh)

			var ch <-chan time.Time
			var s string
			// Try to be smart here: throw away all searches but the
			// last one if there's more typing activity.
			for {
				var loop bool
				select {
				case cur := <-searches:
					s = cur
					ch = time.After(250 * time.Millisecond)
					loop = true
				case <-teardown:
					return
				case <-ch:
				}
				if loop {
					continue
				}
				ui.Update(func() { updateSearch(s) })
				ch = nil
				s = ""
			}
		}()

		input.OnChanged(func(e *tui.Entry) {
			searches <- e.Text()

		})

		text := tui.NewLabel("")
		text.SetWordWrap(true)
		detailView := text

		ui.SetKeybinding("Esc", func() {
			if text.Text() == "" {
				ui.Quit()
			}
			text.SetText("")
			ui.SetWidget(root)
		})
		ui.SetKeybinding("Enter", func() {
			if text.Text() == "" {
				i := table.Selected()
				if i < 0 {
					return
				}
				text.SetText(results[i].URL + "\n\n" + results[i].Comments)
				text.SetWordWrap(true)
				ui.SetWidget(detailView)
			} else {
				text.SetText("")
				ui.SetWidget(root)
			}
		})
		if err := ui.Run(); err != nil {
			return err
		}

		// tw := tablewriter.NewWriter(os.Stdout)
		// tw.SetRowLine(true)

		// tw.SetHeader([]string{"Title", "Excerpt", "URL"})
		// for _, result := range results {
		// 	tw.Append([]string{
		// 		result.Title,
		// 		fmt.Sprintf("%q", result.Excerpt),
		// 		result.URL,
		// 	})
		// }
		// tw.Render()
		fmt.Println()
		return nil
	},
}
