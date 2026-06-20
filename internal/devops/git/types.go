package git

import "time"

// FileStatus represents the git status of a file.
type FileStatus int

// The working-tree states a File can be in, as reported by git status.
const (
	Untracked FileStatus = iota
	Unmodified
	Modified
	Added
	Deleted
	Renamed
)

// File is one file reported by git status: its name and working-tree state.
type File struct {
	Name   string
	Status FileStatus
}

// StatusString returns the file's working-tree state as a lowercase word.
func (f File) StatusString() string {
	switch f.Status {
	case Untracked:
		return "untracked"
	case Unmodified:
		return "unmodified"
	case Modified:
		return "modified"
	case Added:
		return "added"
	case Deleted:
		return "deleted"
	case Renamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// SyncStatus describes how the local branch relates to its upstream.
type SyncStatus struct {
	HasUpstream bool
	Ahead       int
	Behind      int
}

// InSync returns true when an upstream exists and both counters are zero.
func (s SyncStatus) InSync() bool {
	return s.HasUpstream && s.Ahead == 0 && s.Behind == 0
}

// Info is a repository's identity and state for the report header: the current
// branch and commit, how many commits HEAD carries, whether the tree is dirty, its
// sync state, and when HEAD was committed. A zero LastCommit means it could not be
// read (e.g. an empty repo).
type Info struct {
	Branch      string
	Commit      string
	Commits     int
	Dirty       bool
	HasUpstream bool
	Ahead       int
	Behind      int
	LastCommit  time.Time
}

// Repository provides read-only inspection of a git repository: working-tree
// status, upstream sync state, and the identity header.
type Repository interface {
	Status() ([]File, error)
	SyncStatus() (SyncStatus, error)
	Info() (Info, error)
}
