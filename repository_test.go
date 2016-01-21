package git2neo4j_test

import (
	"log"
	"os"
	"os/user"
	"path"
	"testing"

	git "github.com/libgit2/git2go"
	"github.com/nathandao/git2neo4j"
)

// Ensures git2go Repository struct can be created from a Repository struct.
func TestGit2goRepo(t *testing.T) {
	generateTestRepo()
	r := dummyRepository()
	repo, err := r.Git2goRepo()
	if err != nil {
		t.Fatal(err)
	}
	// Last check. If repo is a true repo, it should be freed!
	repo.Free()
}

func TestFetchRemotes(t *testing.T) {
	generateTestRepo()
	r := dummyRepository()
	if err := r.FetchRemotes(); err != nil {
		t.Fatal("Failed to fetch remotes with error", err)
	}
}

// Credentials acts as a helper to test git2neo4j Credentials.
type Credentials struct {
	Username   string
	Publickey  string
	Privatekey string
	Passphrase string
}

const (
	DummyRepoUrl string = "git@github.com:nathandao/git-to-json.git"
)

// generateTestRepo clones a dummy repository to be used for testing.
func generateTestRepo() {
	// Check if dummy_repo exists and only clone if repo does not exist.
	if _, err := os.Stat("test_dir/dummy_repo"); err != nil {
		log.Print("Dummy repository not found, cloning to test_dir/dummy_repo...")
		// Check if dummy_repo is a directory
		cloneOptions := git.CloneOptions{}
		cloneOptions.FetchOptions = &git.FetchOptions{
			RemoteCallbacks: git.RemoteCallbacks{
				CredentialsCallback:      credentialsCallback,
				CertificateCheckCallback: certificateCheckCallback,
			},
		}
		_, err := git.Clone(DummyRepoUrl, "test_dir/", &cloneOptions)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Print("Dummy repository found.")
	}
}

// credentials generates a reusable Credentials struct.
func dummyCredentials() *Credentials {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	uname := "git"
	publickey := path.Join(u.HomeDir, ".ssh/id_rsa.pub")
	privatekey := path.Join(u.HomeDir, ".ssh/id_rsa")
	passphrase := ""
	return &Credentials{
		Username:   uname,
		Publickey:  publickey,
		Privatekey: privatekey,
		Passphrase: passphrase,
	}
}

func credentialsCallback(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	uname := "git"
	publickey := path.Join(u.HomeDir, ".ssh/id_rsa.pub")
	privatekey := path.Join(u.HomeDir, ".ssh/id_rsa")
	passphrase := ""
	ret, cred := git.NewCredSshKey(uname, publickey, privatekey, passphrase)
	return git.ErrorCode(ret), &cred
}

func certificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return 0
}

func dummyRepository() *git2neo4j.Repository {
	cred := dummyCredentials()
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	r := git2neo4j.Repository{
		Path: path.Join(wd, "test_dir/dummy_repo"),
		Credentials: git2neo4j.Credentials{
			Username:   cred.Username,
			Publickey:  cred.Publickey,
			Privatekey: cred.Privatekey,
			Passphrase: cred.Passphrase,
		},
	}
	return &r
}
