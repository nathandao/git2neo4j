package git2neo4j

import (
	"bytes"
	"encoding/csv"
	"os"
	"strconv"
	"strings"

	"github.com/jmcvetta/neoism"
	git "github.com/libgit2/git2go"
)

// Repository contains a path string to the repository, relative to the folder
// the command is run from.
type Repository struct {
	// The repository's unique id.
	Id string
	// Path to the .git file of the repository, which is obtained by using
	// the parameter --mirror when cloning.
	// Eg: git clone --mirror git@github.com:youaccount/repo.git
	Path string
	// Credentials contains the required ssh credentials to perform remote
	Credentials
}

// CertificateCheckCallback is provided as one of the RemoteCallbacks functions
// during fetch.
func (r *Repository) CertificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return 0
}

// CredentialsCallback is provided as one of the RemoteCallbacks functions
// during fetch.
func (r *Repository) CredentialsCallback(url string, uname_from_url string, allowed_types git.CredType) (git.ErrorCode, *git.Cred) {
	return r.Credentials.LibgitCred()
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
	defer csvFile.Close()
	err = w.Write([]string{
		"hash", "message", "author_name", "author_email", "author_time",
		"author_timestamp", "commit_time", "commit_timestamp", "parents",
	})
	if err != nil {
		return err
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return err
	}
	// Iterate the walk.
	err = revWalk.Iterate(revWalkHandler)
	if err != nil {
		return err
	}
	return nil
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

// Git2goRepo returns the git2go.Repository struct from the Repository's path.
func (r *Repository) Git2goRepo() (*git.Repository, error) {
	repo, err := git.OpenRepository(r.Path)
	return repo, err
}

// ImportGraph issues a neo4j cypher to import the exported csv into neo4j.
func (v *Repository) ImportGraph() error {
	// Export data to csv file.
	if err := v.ExportToCsv(); err != nil {
		return err
	}
	// Connect to Neo4j.
	settings := GetSettings()
	db, err := neoism.Connect(settings.DbUrl)
	if err != nil {
		return err
	}
	// Generate constraints
	constraints := []neoism.CypherQuery{
		neoism.CypherQuery{
			Statement: `CREATE CONSTRAINT ON (r:Repository) ASSERT r.id IS UNIQUE`,
		},
		neoism.CypherQuery{
			Statement: `CREATE CONSTRAINT ON (c:Commit) ASSERT c.hash IS UNIQUE`,
		},
		neoism.CypherQuery{
			Statement: `CREATE CONSTRAINT ON (a:Author) ASSERT a.email IS UNIQUE`,
		},
	}
	for _, constraintCq := range constraints {
		if err = db.Cypher(&constraintCq); err != nil {
			return err
		}
	}
	// Get repo struct.
	r, err := v.Git2goRepo()
	if err != nil {
		return err
	}
	// Create repo node.
	cq := neoism.CypherQuery{
		Statement: `MERGE (r:Repository {id: {id}, path: {path}})`,
		Parameters: neoism.Props{
			"id":   v.Id,
			"path": r.Path(),
		},
	}
	if err = db.Cypher(&cq); err != nil {
		return err
	}
	// Get csv file path.
	var buffer bytes.Buffer
	csvFilePath, err := TemporaryCsvPath(r)
	if err != nil {
		return err
	}
	buffer.WriteString("file://")
	buffer.WriteString(csvFilePath)
	csvFilePath = buffer.String()
	// Construct cypher query to import CSV.
	cq = neoism.CypherQuery{
		Statement: `
USING PERIODIC COMMIT 1000
LOAD CSV WITH headers FROM {csv_file} as line

MATCH (r:Repository {id: {id}})
MERGE (c:Commit {hash: line.hash}) ON CREATE SET
  c.message = line.message,
  c.author_time = line.author_time,
  c.author_timestamp = toInt(line.author_timestamp),
  c.commit_time = line.commit_time,
  c.commit_timestamp = toInt(line.commit_timestamp),
  c.parents = split(line.parents, ' ')

MERGE (r)-[:HAS_COMMIT]->(c)

MERGE (u:User:Author {email:line.author_email}) ON CREATE SET u.name = line.author_name
MERGE (u)-[:AUTHORED]->(c)
MERGE (c)-[:AUTHORED_BY]->(u)
MERGE (u)-[:CONTRIBUTED_TO]->(r)

WITH c,line
WHERE line.parents <> ''
FOREACH (parent_hash in split(line.parents, ' ') |
  MERGE (parent:Commit {hash: parent_hash})
  MERGE (c)-[:HAS_PARENT]->(parent))
`,
		Parameters: neoism.Props{
			"csv_file": csvFilePath,
			"id":       v.Id,
		},
	}
	if err = db.Cypher(&cq); err != nil {
		return err
	}
	return nil
}

// importBranchGraph fetches the latest remote branches and add the according
// relationthips to their target commits.
func (v *Repository) ImportBranchGraph() error {
	r, err := v.Git2goRepo()
	if err != nil {
		return err
	}
	defer r.Free()
	// Connect to Neo4j.
	settings := GetSettings()
	db, err := neoism.Connect(settings.DbUrl)
	// Remove all current branches before re-fetching remote branches.
	cq := neoism.CypherQuery{
		Statement: `MATCH (:Repository {id: {id}})-[:HAS_BRANCH]->(b) DETACH DELETE b`,
		Parameters: neoism.Props{
			"id": v.Id,
		},
	}
	if err = db.Cypher(&cq); err != nil {
		return err
	}
	// Add all remote branches to the graph.
	referenceIterator, err := r.NewReferenceIterator()
	if err != nil {
		return err
	}
	for {
		reference, err := referenceIterator.Next()
		if err != nil {
			break
		}
		if isRemote := reference.IsRemote(); isRemote == true {
			if target := reference.Target(); target != nil {
				// Get branch name.
				branch := reference.Branch()
				branchName, err := branch.Name()
				if err != nil {
					return err
				}
				// Get branch last commit target.
				hash := reference.Target().String()
				// Add branch to graph db.
				cq := neoism.CypherQuery{
					Statement: `
MATCH (r:Repository {id : {id}})
WITH r
MATCH (c:Commit {hash: {hash}})
WITH r, c
CREATE (r)-[:HAS_BRANCH]->(:Branch {name: {name}})-[:POINTS_TO]->(c)
`,
					Parameters: neoism.Props{
						"id":   v.Id,
						"hash": hash,
						"name": branchName,
					},
				}
				if err = db.Cypher(&cq); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// SyncRemotes checks if there are new commits after fetching new remote data,
// and update the graph accordingly. If there are new commits, return true.
func (v *Repository) SyncRemotes() error {
	// Fetch remotes and go through the updated revwalk.
	if err := v.FetchRemotes(); err != nil {
		return err
	}
	r, err := v.Git2goRepo()
	if err != nil {
		return err
	}
	defer r.Free()
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
				// Initiate the walk.
				revWalk, err := r.Walk()
				if err != nil {
					return err
				}
				defer revWalk.Free()
				if err = revWalk.Push(target); err != nil {
					return err
				}
				// Iterate the walk.
				revWalk.Sorting(git.SortTime)
				err = revWalk.Iterate(revWalkSyncHandler)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// TemoraryCsvPath returns the repository's path to the temp folder.
func TemporaryCsvPath(r *git.Repository) (string, error) {
	// Create unique csv file name through path.
	fileName := strings.Replace(r.Path(), "/", "_", -1)
	var buffer bytes.Buffer
	// Current path
	currentPath, err := os.Getwd()
	if err != nil {
		return "", err
	}
	buffer.WriteString(currentPath)
	buffer.WriteString("/tmp/")
	buffer.WriteString(fileName)
	buffer.WriteString(".csv")
	return buffer.String(), nil
}

func remoteBranchIteratorHandler(branch *git.Branch, branchType git.BranchType) error {
	return nil
}

// revWalkHandler goes through each commit and creates relevant info for each
// commit.
func revWalkHandler(commit *git.Commit) bool {
	// Commit info.
	hash := commit.Id().String()
	if hash == "" {
		return false
	}
	message := strings.Replace(commit.Message(), "\"", "'", -1)
	// Parent info.
	parentCount := commit.ParentCount()
	var buffer bytes.Buffer
	for i := uint(0); i < parentCount; i = i + 1 {
		buffer.WriteString(commit.ParentId(i).String())
		if i < parentCount-1 {
			buffer.WriteString(" ")
		}
	}
	parents := buffer.String()
	// Author info.
	authorSig := commit.Author()
	authorName := authorSig.Name
	authorEmail := authorSig.Email
	authorTime := authorSig.When.Format("2006-01-02 15:04:05 +0000")
	authorTimestamp := strconv.Itoa(int(authorSig.When.Unix()))
	// Committer info.
	committerSig := commit.Committer()
	commitTime := committerSig.When.Format("2006-01-02 15:04:05 +0000")
	commitTimestamp := strconv.Itoa(int(committerSig.When.Unix()))
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
		hash, message, authorName, authorEmail, authorTime, authorTimestamp,
		commitTime, commitTimestamp, parents,
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

// revWalkSyncHandler adds new commits to the repo graph.
// The walk stops when reaching the previous lastest commit in the database.
func revWalkSyncHandler(commit *git.Commit) bool {
	// Get repo path.
	r := commit.Owner()
	path := r.Path()
	// Commit info.
	hash := commit.Id().String()
	settings := GetSettings()
	db, err := neoism.Connect(settings.DbUrl)
	if err != nil {
		panic(err)
	}
	res := []struct {
		Hash string `json:"c.hash"`
	}{}
	cq := neoism.CypherQuery{
		Statement: `
MATCH (:Repository {path: {path}})-[:HAS_COMMIT]->(c:Commit {hash: {hash}})
RETURN c.hash
`,
		Parameters: neoism.Props{
			"hash": hash,
			"path": path,
		},
		Result: &res,
	}
	if err = db.Cypher(&cq); err != nil {
		panic(err)
	}
	// When reaching an existing commit in the database, stop the walk.
	if size := len(res); size > 0 {
		return false
	}
	// Else, add the commit to the database.
	message := strings.Replace(commit.Message(), "\"", "'", -1)
	// Parent info.
	parentCount := commit.ParentCount()
	var buffer bytes.Buffer
	for i := uint(0); i < parentCount; i = i + 1 {
		buffer.WriteString(commit.ParentId(i).String())
		if i < parentCount-1 {
			buffer.WriteString(" ")
		}
	}
	parents := buffer.String()
	// Author info.
	authorSig := commit.Author()
	authorName := authorSig.Name
	authorEmail := authorSig.Email
	authorTime := authorSig.When.Format("2006-01-02 15:04:05 +0000")
	authorTimestamp := strconv.Itoa(int(authorSig.When.Unix()))
	// Committer info.
	committerSig := commit.Committer()
	commitTime := committerSig.When.Format("2006-01-02 15:04:05 +0000")
	commitTimestamp := strconv.Itoa(int(committerSig.When.Unix()))
	// Add accoding info to the graph.
	commitCq := neoism.CypherQuery{
		Statement: `
MATCH (r:Repository {path: {path}})
CREATE (r)-[:HAS_COMMIT]->(c:Commit {
  message: {message},
  author_time: {author_time},
  author_timestamp: toInt({author_timestamp}),
  commit_time: {commit_time},
  commit_timestamp: toInt({commit_timestamp}),
  hash: {hash},
  parents: split({parents}, ' ')})

MERGE (u:User:Author {email: {author_email}}) ON CREATE SET u.name = {author_name}
MERGE (u)-[:AUTHORED]->(c)
MERGE (c)-[:AUTHORED_BY]->(u)
MERGE (u)-[:CONTRIBUTED_TO]->(r)

WITH c
WHERE {parents} <> ''
FOREACH (parent_hash in split({parents}, ' ') |
  MERGE (parent:Commit {hash: parent_hash})
  MERGE (c)-[:HAS_PARENT]->(parent))
`,
		Parameters: neoism.Props{
			"path":             path,
			"hash":             hash,
			"message":          message,
			"author_name":      authorName,
			"author_email":     authorEmail,
			"author_time":      authorTime,
			"author_timestamp": authorTimestamp,
			"commit_time":      commitTime,
			"commit_timestamp": commitTimestamp,
			"parents":          parents,
		},
	}
	if err = db.Cypher(&commitCq); err != nil {
		return false
	}
	return true
}
