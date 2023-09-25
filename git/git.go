package git

import (
	"fmt"
	"os/exec"
)

type Repository struct {
	Repository string
	BaseFolder string
	Folder string
}

func NewGit(folder string, repo string) *Repository{
	g:= &Repository{
		Repository: repo,
		BaseFolder: folder,
		Folder: "base",
	}

	cmd := exec.Command(fmt.Sprintf("git clone %s %s", repo, folder))
	cmd.Dir = g.BaseFolder
	cmd.Run()

	return g
}

func (g *Repository) AddAll() error{
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	return cmd.Run()
}

func (g *Repository) Commit(message string) error{
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	return cmd.Run()
}

func (g *Repository) Push() error{
	cmd := exec.Command("git", "push")
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	return cmd.Run()
}

func (g *Repository) Pull() error{
	cmd := exec.Command("git", "pull")
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	return cmd.Run()
}
