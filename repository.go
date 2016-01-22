package git2neo4j

import (
	"fmt"

	git "github.com/libgit2/git2go"
)

// Repository contains a path string to the repository, relative to the folder
// the command is run from.
type Repository struct {
	// Path to the .git file of the repository, which is obtained by using
	// the parameter --mirror when cloning.
	// Eg: git clone --mirror git@github.com:youaccount/repo.git
	Path string
	// Credentials contains the required ssh credentials to perform remote
	Credentials
}

// Git2goRepo returns the git2go.Repository struct from the Repository's path.
func (r *Repository) Git2goRepo() (*git.Repository, error) {
	repo, err := git.OpenRepository(r.Path)
	return repo, err
}

// FetchRemotes executes fetch from remotes. This will update info of all
// remote refspecs.
func (v *Repository) FetchRemotes() error {
	r, err := v.Git2goRepo()
	if err != nil {
		return err
	}
	defer r.Free()
	// Iterate through all remotes and perform fetches.
	rnames, err := r.Remotes.List()
	if err != nil {
		return err
	}
	// Remote callbacks that contains required ssh credentials.
	remotecb := git.RemoteCallbacks{
		CredentialsCallback:      v.CredentialsCallback,
		CertificateCheckCallback: v.CertificateCheckCallback,
	}
	// Fetch options.
	fetchoptions := git.FetchOptions{
		RemoteCallbacks: remotecb,
		DownloadTags:    git.DownloadTagsAuto,
		Prune:           git.FetchPruneOn,
	}
	for _, rname := range rnames {
		remote, err := r.Remotes.Lookup(rname)
		if err != nil {
			return err
		}
		defer remote.Free()
		// Get refspecs list to be fetched.
		refspecs, err := remote.FetchRefspecs()
		if err != nil {
			return err
		}
		// Perform the fetch.
		if err = remote.Fetch(refspecs, &fetchoptions, ""); err != nil {
			return err
		}
	}
	return nil
}

func (v *Repository) Test() error {
	r, err := v.Git2goRepo()
	if err != nil {
		return err
	}
	defer r.Free()
	// Initiate the walk.
	revWalk, err := r.Walk()
	if err != nil {
		return err
	}
	defer revWalk.Free()
	// Iterate through all refs to get the starting Oid for the walk.
	referenceIterator, err := r.NewReferenceIterator()
	if err != nil {
		return err
	}
	defer referenceIterator.Free()
	for {
		reference, err := referenceIterator.Next()
		if err != nil {
			break
		}
		defer reference.Free()
		if isRemote := reference.IsRemote(); isRemote == true {
			if target := reference.Target(); target != nil {
				if err = revWalk.Push(target); err != nil {
					return err
				}
			}
		}
	}
	err = revWalk.Iterate(revWalkHandler)
	if err != nil {
		return err
	}
	return nil
}

// CredentialsCallback is provided as one of the RemoteCallbacks functions
// during fetch.
func (r *Repository) CredentialsCallback(url string, uname_from_url string, allowed_types git.CredType) (git.ErrorCode, *git.Cred) {
	return r.Credentials.LibgitCred()
}

// CertificateCheckCallback is provided as one of the RemoteCallbacks functions
// during fetch.
func (r *Repository) CertificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return 0
}

func remoteBranchIteratorHandler(branch *git.Branch, branchType git.BranchType) error {
	return nil
}

func revWalkHandler(commit *git.Commit) bool {
	// Get current commit's tree and its first parent's tree.
	commitTree, err := commit.Tree()
	if err != nil {
		panic(err)
	}
	if parent := commit.Parent(0); parent != nil {
		parentTree, err := parent.Tree()
		if err != nil {
			panic(err)
		}
		// Compare the 2 trees to get diff stats.
		r := commit.Owner()
		diff, err := r.DiffTreeToTree(parentTree, commitTree, nil)
		if err != nil {
			panic(err)
		}
		defer diff.Free()
		diffStat, err := diff.Stats()
		if err != nil {
			panic(err)
		}
		defer diffStat.Free()
		deletions := diffStat.Deletions()
		insertions := diffStat.Insertions()
		fmt.Println("Additions:", insertions)
		fmt.Println("Deletions:", deletions)
	}
	return true
}
