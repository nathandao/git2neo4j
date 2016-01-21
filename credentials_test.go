package git2neo4j_test

import (
	. "github.com/nathandao/git2neo4j"
	"os/user"
	"path"
	"testing"

	git "github.com/libgit2/git2go"
)

func TestLibgitCred(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	// TODO: Read these from conf file
	publickey := path.Join(u.HomeDir, ".ssh/id_rsa.pub")
	privatekey := path.Join(u.HomeDir, ".ssh/id_rsa")
	passphrase := ""
	c := Credentials{
		Username:   "git",
		Publickey:  publickey,
		Privatekey: privatekey,
		Passphrase: passphrase,
	}
	ret, _ := c.LibgitCred()
	if ret != git.ErrOk {
		t.Fatal("Failed to create git2go credentials")
	}
}
