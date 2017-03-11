package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/dforsyth/reflectclient-github"
	"github.com/dforsyth/reflectclient-github/models"
)

type ctx struct {
	username string
	gh       *github.GithubServiceV3
}

func (ctx *ctx) updateForked(repo *models.Repo) error {
	forked, err := ctx.gh.Repo(&github.RepoParams{Owner: ctx.username, Repo: repo.Name})
	if err != nil {
		return err
	}

	origDir, err := os.Getwd()
	if err != nil {
		return err
	}
	cloneDir := path.Join(os.TempDir(), fmt.Sprintf("forkupdate-%s", repo.Name))

	os.RemoveAll(cloneDir)

	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(cloneDir)
	}()

	log.Printf("Updating %s (%s)...", forked.FullName, forked.Parent.FullName)

	clone := exec.Command("git", "clone", "--depth", "1", fmt.Sprintf("git@github.com:%s.git", repo.FullName), cloneDir)
	if err := clone.Run(); err != nil {
		return err
	}

	if err := os.Chdir(cloneDir); err != nil {
		return err
	}

	remote := exec.Command("git", "remote", "add", "upstream", fmt.Sprintf("https://github.com/%s.git", forked.Parent.FullName))
	if err := remote.Run(); err != nil {
		return err
	}

	fetch := exec.Command("git", "fetch", "upstream", repo.DefaultBranch)
	if err := fetch.Run(); err != nil {
		return err
	}

	reset := exec.Command("git", "reset", "--hard", fmt.Sprintf("upstream/%s", repo.DefaultBranch))
	if err := reset.Run(); err != nil {
		return err
	}

	push := exec.Command("git", "push", "origin", repo.DefaultBranch)
	if err := push.Run(); err != nil {
		return err
	}

	log.Print("Done.")

	return nil
}

func setupContext() (*ctx, error) {
	ctx := new(ctx)

	if username := os.Getenv("GITHUB_USERNAME"); username != "" {
		ctx.username = username
	} else {
		flag.StringVar(&ctx.username, "username", "", "username")
	}

	var token string
	if token = os.Getenv("GITHUB_TOKEN"); token == "" {
		flag.StringVar(&token, "token", "", "token")
	}

	flag.Parse()

	if ctx.username == "" {
		return nil, errors.New("no username")
	}
	if token == "" {
		return nil, errors.New("no token")
	}

	gh, err := github.MakeService("https://api.github.com", github.MakeDefaultTokenProvider(ctx.username, token))
	if err != nil {
		return nil, err
	}
	ctx.gh = gh

	return ctx, nil
}

func main() {
	ctx, err := setupContext()
	if err != nil {
		log.Fatal(err)
	}

	repos, err := ctx.gh.UserRepos(&github.UserReposParams{UserName: ctx.username})
	if err != nil {
		log.Fatal(err)
	}

	for _, repo := range repos {
		if !repo.Fork {
			continue
		}

		if err := ctx.updateForked(repo); err != nil {
			log.Println(err)
		}
	}
}
