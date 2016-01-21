package git2neo4j

import (
	git "github.com/libgit2/git2go"
)

// Credentials store the required ssh credentials info to access the git
// repository.
type Credentials struct {
	Username   string
	Publickey  string
	Privatekey string
}

func (c *Credentials) LibgitCred() (git.ErrorCode, *git.Cred) {
	ret, cred := git.NewCredSshKey("git", "/Users/nathan/.ssh/id_rsa.pub", "/Users/nathan/.ssh/id_rsa", "")
	return git.ErrorCode(ret), &cred
}
