package git2neo4j

import (
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
