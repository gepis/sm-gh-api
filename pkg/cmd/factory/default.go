package factory

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gepis/sm-gh-api/api"
	"github.com/gepis/sm-gh-api/context"
	"github.com/gepis/sm-gh-api/git"
	"github.com/gepis/sm-gh-api/core/config"
	"github.com/gepis/sm-gh-api/core/ghrepo"
	"github.com/gepis/sm-gh-api/pkg/cmdutil"
	"github.com/gepis/sm-gh-api/pkg/iostreams"
)

func New() *cmdutil.Factory {
	f := &cmdutil.Factory{
		Config:     configFunc(), // No factory dependencies
		Branch:     branchFunc(), // No factory dependencies
		Executable: executable(), // No factory dependencies
	}

	f.IOStreams = ioStreams(f)                   // Depends on Config
	f.HttpClient = httpClientFunc(f, "x")        // Depends on Config, IOStreams, and appVersion
	f.Remotes = remotesFunc(f)                   // Depends on Config
	f.BaseRepo = BaseRepoFunc(f)                 // Depends on Remotes
	f.Browser = browser(f)                       // Depends on Config, and IOStreams

	return f
}

func BaseRepoFunc(f *cmdutil.Factory) func() (ghrepo.Interface, error) {
	return func() (ghrepo.Interface, error) {
		remotes, err := f.Remotes()
		if err != nil {
			return nil, err
		}

		return remotes[0], nil
	}
}

func SmartBaseRepoFunc(f *cmdutil.Factory) func() (ghrepo.Interface, error) {
	return func() (ghrepo.Interface, error) {
		httpClient, err := f.HttpClient()
		if err != nil {
			return nil, err
		}

		apiClient := api.NewClientFromHTTP(httpClient)

		remotes, err := f.Remotes()
		if err != nil {
			return nil, err
		}

		repoContext, err := context.ResolveRemotesToRepos(remotes, apiClient, "")
		if err != nil {
			return nil, err
		}

		baseRepo, err := repoContext.BaseRepo(f.IOStreams)
		if err != nil {
			return nil, err
		}

		return baseRepo, nil
	}
}

func remotesFunc(f *cmdutil.Factory) func() (context.Remotes, error) {
	rr := &remoteResolver{
		readRemotes: git.Remotes,
		getConfig:   f.Config,
	}

	return rr.Resolver()
}

func httpClientFunc(f *cmdutil.Factory, appVersion string) func() (*http.Client, error) {
	return func() (*http.Client, error) {
		io := f.IOStreams
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}

		return NewHTTPClient(io, cfg, appVersion, true)
	}
}

func browser(f *cmdutil.Factory) cmdutil.Browser {
	io := f.IOStreams
	return cmdutil.NewBrowser(browserLauncher(f), io.Out, io.ErrOut)
}

// Browser precedence
// 1. GH_BROWSER
// 2. browser from config
// 3. BROWSER
func browserLauncher(f *cmdutil.Factory) string {
	if smBrowser := os.Getenv("GH_BROWSER"); smBrowser != "" {
		return smBrowser
	}

	cfg, err := f.Config()
	if err == nil {
		if cfgBrowser, _ := cfg.Get("", "browser"); cfgBrowser != "" {
			return cfgBrowser
		}
	}

	return os.Getenv("BROWSER")
}

func executable() string {
	secman := "secman"
	if exe, err := os.Executable(); err == nil {
		secman = exe
	}

	return secman
}

func configFunc() func() (config.Config, error) {
	var cachedConfig config.Config
	var configError error
	return func() (config.Config, error) {
		if cachedConfig != nil || configError != nil {
			return cachedConfig, configError
		}

		cachedConfig, configError = config.ParseDefaultConfig()
		if errors.Is(configError, os.ErrNotExist) {
			cachedConfig = config.NewBlankConfig()
			configError = nil
		}

		cachedConfig = config.InheritEnv(cachedConfig)
		return cachedConfig, configError
	}
}

func branchFunc() func() (string, error) {
	return func() (string, error) {
		currentBranch, err := git.CurrentBranch()
		if err != nil {
			return "", fmt.Errorf("could not determine current branch: %w", err)
		}

		return currentBranch, nil
	}
}

func ioStreams(f *cmdutil.Factory) *iostreams.IOStreams {
	io := iostreams.System()
	cfg, err := f.Config()
	if err != nil {
		return io
	}

	if prompt, _ := cfg.Get("", "prompt"); prompt == "disabled" {
		io.SetNeverPrompt(true)
	}

	// Pager precedence
	// 1. GH_PAGER
	// 2. pager from config
	// 3. PAGER
	if ghPager, ghPagerExists := os.LookupEnv("GH_PAGER"); ghPagerExists {
		io.SetPager(ghPager)
	} else if pager, _ := cfg.Get("", "pager"); pager != "" {
		io.SetPager(pager)
	}

	return io
}
