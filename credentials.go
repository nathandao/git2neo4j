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
	Passphrase string
}

// LibgitCred returns a git2go *Cred struct, which is used for remote
// operations over ssh.
func (c *Credentials) LibgitCred() (git.ErrorCode, *git.Cred) {
	returnCode, credentials := git.NewCredSshKey(c.Username, c.Publickey, c.Privatekey, c.Passphrase)
	return git.ErrorCode(returnCode), &credentials
}
