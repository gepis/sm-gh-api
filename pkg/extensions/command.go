package extensions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gepis/sm-gh-api/core/ghrepo"
	"github.com/gepis/sm-gh-api/pkg/cmdutil"
	"github.com/gepis/sm-gh-api/pkg/iostreams"
	"github.com/spf13/cobra"
)

func NewCmdExtensions(f *cmdutil.Factory) *cobra.Command {
	m := f.ExtensionManager
	io := f.IOStreams

	extCmd := cobra.Command{
		Use:   "extensions",
		Short: "Manage secman github-api extensions",
		Long: heredoc.Docf(`
			GitHub CLI extensions are repositories that provide additional gh commands.
			The name of the extension repository must start with "gh-" and it must contain an
			executable of the same name. All arguments passed to the %[1]sgh <extname>%[1]s invocation
			will be forwarded to the %[1]sgh-<extname>%[1]s executable of the extension.
			An extension cannot override any of the core gh commands.
		`, "`"),
	}

	extCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List installed extension commands",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cmds := m.List()
				if len(cmds) == 0 {
					return errors.New("no extensions installed")
				}
				// cs := io.ColorScheme()
				t := utils.NewTablePrinter(io)
				for _, c := range cmds {
					var repo string
					if u, err := git.ParseURL(c.URL()); err == nil {
						if r, err := ghrepo.FromURL(u); err == nil {
							repo = ghrepo.FullName(r)
						}
					}

					t.AddField(fmt.Sprintf("gh %s", c.Name()), nil, nil)
					t.AddField(repo, nil, nil)
					// TODO: add notice about available update
					//t.AddField("Update available", nil, cs.Green)
					t.EndRow()
				}

				return t.Render()
			},
		},
		&cobra.Command{
			Use:   "install <repo>",
			Short: "Install a gh extension from a repository",
			Args:  cmdutil.MinimumArgs(1, "must specify a repository to install from"),
			RunE: func(cmd *cobra.Command, args []string) error {
				if args[0] == "." {
					wd, err := os.Getwd()
					if err != nil {
						return err
					}

					return m.InstallLocal(wd)
				}

				repo, err := ghrepo.FromFullName(args[0])
				if err != nil {
					return err
				}

				if !strings.HasPrefix(repo.RepoName(), "gh-") {
					return errors.New("the repository name must start with `gh-`")
				}

				cfg, err := f.Config()
				if err != nil {
					return err
				}

				protocol, _ := cfg.Get(repo.RepoHost(), "git_protocol")
				return m.Install(ghrepo.FormatRemoteURL(repo, protocol), io.Out, io.ErrOut)
			},
		},
		func() *cobra.Command {
			var flagAll bool
			cmd := &cobra.Command{
				Use:   "upgrade {<name> | --all}",
				Short: "Upgrade installed extensions",
				Args: func(cmd *cobra.Command, args []string) error {
					if len(args) == 0 && !flagAll {
						return &cmdutil.FlagError{Err: errors.New("must specify an extension to upgrade")}
					}

					if len(args) > 0 && flagAll {
						return &cmdutil.FlagError{Err: errors.New("cannot use `--all` with extension name")}
					}

					if len(args) > 1 {
						return &cmdutil.FlagError{Err: errors.New("too many arguments")}
					}

					return nil
				},
				RunE: func(cmd *cobra.Command, args []string) error {
					var name string
					if len(args) > 0 {
						name = args[0]
					}

					return m.Upgrade(name, io.Out, io.ErrOut)
				},
			}
			cmd.Flags().BoolVar(&flagAll, "all", false, "Upgrade all extensions")
			return cmd
		}(),
		&cobra.Command{
			Use:   "remove",
			Short: "Remove an installed extension",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return m.Remove(args[0])
			},
		},
	)

	extCmd.Hidden = true

	return &extCmd
}
