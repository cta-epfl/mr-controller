package git

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
)

type Repository struct {
	Repository string
	Branch     string
	Folder     string
}

func NewGit(folder string, repo string, branch string) (*Repository, error) {
	g := &Repository{
		Repository: repo,
		Branch:     branch,
		Folder:     folder,
	}

	cmd := exec.Command("git", "config", "--global", "user.name", "mrcontroller[bot]")
	runCommand(cmd)
	cmd = exec.Command("git", "config", "--global", "user.email", "mrcontroller[bot]@epfl.ch")
	runCommand(cmd)

	cmd = exec.Command("git", "clone", repo, folder, "--branch", g.Branch)
	cmd.Dir = g.Folder
	err := runCommand(cmd)
	return g, err
}

func (g *Repository) AddAll() error {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = g.Folder
	return runCommand(cmd)
}

func (g *Repository) Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", "\""+message+"\"")
	cmd.Dir = g.Folder
	return runCommand(cmd)
}

func (g *Repository) Push() error {
	cmd := exec.Command("git", "push", "-u", "origin", g.Branch)
	cmd.Dir = g.Folder
	return runCommand(cmd)
}

func (g *Repository) Pull() error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = g.Folder
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) error {
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		log.Printf("out: %q ; err: %q", strings.TrimSpace(outb.String()), strings.TrimSpace(errb.String()))
	}
	return err
}
