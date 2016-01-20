package git2neo4j

import (
	"log"
	"os"
	"path/filepath"

	"github.com/lemmi/git"
)

// Repository contains a path string to the repository, relative to the folder
// the command is run from.
type Repository struct {
	// Path to the .git file of the repository, which is obtained by using
	// the parameter --mirror when cloning.
	// Eg: git clone --mirror git@your_repo.git
	Path string
}

// Getinfo retrieve all the required information about the repository and return
// a RepositoryInfo type.
func (rp *Repository) GetInfo() ([]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	repository, err := git.OpenRepository(filepath.Join(wd, rp.Path))
	if err != nil {
		log.Fatal(err)
	}
	cc, err := repository.GetBranches()
	if err != nil {
		return nil, err
	}
	return cc, nil
}
