package git2neo4j

import (
	"bytes"
	"encoding/csv"
	"os"
	"strings"

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

// ExportToCsv exports the repository commits data to a csv file inside the
// tmp folder.
func (v *Repository) ExportToCsv() error {
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
	// Create a new temporary csv export file
	csvPath, err := TemporaryCsvPath(r)
	if err != nil {
		return err
	}
	csvFile, err := os.Create(csvPath)
	if err != nil {
		return err
	}
	w := csv.NewWriter(csvFile)
	err = w.Write([]string{
		"hash", "message", "authorName", "authorEmail", "authorTime",
	})
	csvFile.Close()
	// Iterate the walk.
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

// revWalkHandler goes through each commit and creates relevant info for each
// commit.
func revWalkHandler(commit *git.Commit) bool {
	// Commit info.
	oid := commit.Id()
	hash := oid.String()
	message := commit.Message()
	// Author info.
	authorSig := commit.Author()
	authorName := authorSig.Name
	authorEmail := authorSig.Email
	authorTime := authorSig.When.Format("2006-01-02 15:04:05 +0800")
	// Write to csvFile
	r := commit.Owner()
	csvPath, err := TemporaryCsvPath(r)
	if err != nil {
		panic(err)
	}
	csvFile, err := os.OpenFile(csvPath, os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer csvFile.Close()
	w := csv.NewWriter(csvFile)
	err = w.Write([]string{
		hash, message, authorName, authorEmail, authorTime,
	})
	if err != nil {
		panic(err)
	}
	w.Flush()
	if err = w.Error(); err != nil {
		panic(err)
	}
	return true
}

// TemoraryCsvPath returns the repository's path to the temp folder.
func TemporaryCsvPath(r *git.Repository) (string, error) {
	// Create unique csv file name through path.
	var buffer bytes.Buffer
	fileName := strings.Replace(r.Path(), "/", "_", -1)
	buffer.WriteString("./tmp/")
	buffer.WriteString(fileName)
	buffer.WriteString(".csv")
	return buffer.String(), nil
}
