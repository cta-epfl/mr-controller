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

func (g *Repository) AddAll() {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	cmd.Run()
}

func (g *Repository) Commit(message string) {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	cmd.Run()
}

func (g *Repository) Push() {
	cmd := exec.Command("git", "push")
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	cmd.Run()
}

func (g *Repository) Pull() {
	cmd := exec.Command("git", "pull")
	cmd.Dir = g.BaseFolder+"/"+g.Folder
	cmd.Run()
}
