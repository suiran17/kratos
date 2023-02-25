package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/go-kratos/kratos/cmd/kratos/v2/internal/base"
)

// CmdNew represents the new command.
var CmdNew = &cobra.Command{
	Use:   "new",
	Short: "Create a service template",
	Long:  "Create a service project using the repository template. Example: kratos new helloworld",
	Run:   run,
}

var (
	repoURL string
	branch  string
	timeout string
	nomod   bool
)

func init() {
	if repoURL = os.Getenv("KRATOS_LAYOUT_REPO"); repoURL == "" {
		repoURL = "https://github.com/go-kratos/kratos-layout.git"
	}
	timeout = "60s"
	CmdNew.Flags().StringVarP(&repoURL, "repo-url", "r", repoURL, "layout repo")
	CmdNew.Flags().StringVarP(&branch, "branch", "b", branch, "repo branch")
	CmdNew.Flags().StringVarP(&timeout, "timeout", "t", timeout, "time out")
	CmdNew.Flags().BoolVarP(&nomod, "nomod", "", nomod, "retain go mod")
}

func run(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	t, err := time.ParseDuration(timeout)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	name := ""
	if len(args) == 0 {
		prompt := &survey.Input{
			Message: "What is project name ?",
			Help:    "Created project name.",
		}
		err = survey.AskOne(prompt, &name)
		if err != nil || name == "" {
			return
		}
	} else {
		name = args[0]
	}

	wd = getProjectPlaceDir(name, wd)
	p := &Project{Name: filepath.Base(name), Path: name}
	done := make(chan error, 1)

	go func() {
		// 非 mod 模式
		if !nomod {
			done <- p.New(ctx, wd, repoURL, branch)
			return
		}

		if _, e := os.Stat(path.Join(wd, "go.mod")); os.IsNotExist(e) {
			done <- fmt.Errorf("🚫 go.mod don't exists in %s", wd)
			return
		}

		mod, e := base.ModulePath(path.Join(wd, "go.mod"))
		if e != nil {
			panic(e)
		}

		// done 的缓存区长度为1, 所以这里不会阻塞
		done <- p.Add(ctx, wd, repoURL, branch, mod)
	}()

	select {
	// 超时控制
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			fmt.Fprint(os.Stderr, "\033[31mERROR: project creation timed out\033[m\n")
			return
		}
		fmt.Fprintf(os.Stderr, "\033[31mERROR: failed to create project(%s)\033[m\n", ctx.Err().Error())
	case err = <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mERROR: Failed to create project(%s)\033[m\n", err.Error())
		}
	}
}

// 获取项目地点目录
func getProjectPlaceDir(projectName string, fallbackPlaceDir string) string {
	projectFullPath := projectName

	wd := filepath.Dir(projectName)
	// check for home dir
	if strings.HasPrefix(wd, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// cannot get user home return fallback place dir
			return fallbackPlaceDir
		}
		projectFullPath = filepath.Join(homeDir, projectName[2:])
	}
	// check path is relative
	if !filepath.IsAbs(projectFullPath) {
		absPath, err := filepath.Abs(projectFullPath)
		if err != nil {
			return fallbackPlaceDir
		}
		projectFullPath = absPath
	}
	// create project logic will check stat,so not check path stat here
	return filepath.Dir(projectFullPath)
}
