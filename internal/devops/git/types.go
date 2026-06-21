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

// File is one file reported by git status: its working-tree state, its basename
// and repo-root-relative path, when it was last touched on disk, and how many lines
// it added/deleted relative to HEAD. ModTime is the filesystem mtime of the
// working-tree file; it is zero for a deleted file (no file to stat) or when the
// stat fails. Added/Deleted are the uncommitted line delta (an untracked file counts
// every line as added); both are zero for a binary file or when the diff is unknown.
type File struct {
	Name    string
	Path    string
	Status  FileStatus
	ModTime time.Time
	Added   int
	Deleted int
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
// sync state, when HEAD was committed, and when the working tree was last touched. A
// zero CommittedAt means it could not be read (e.g. an empty repo); a zero TouchedAt
// means the tree is clean (no uncommitted file to date).
type Info struct {
	Branch      string
	Commit      string
	Commits     int
	Dirty       bool
	HasUpstream bool
	Ahead       int
	Behind      int
	CommittedAt time.Time
	TouchedAt   time.Time
	// LinesAdded / LinesDeleted are the uncommitted line delta summed across the
	// working tree (the changes a single commit would record against HEAD).
	LinesAdded   int
	LinesDeleted int
}

// Repository provides read-only inspection of a git repository: working-tree
// status, upstream sync state, and the identity header.
type Repository interface {
	Status() ([]File, error)
	SyncStatus() (SyncStatus, error)
	Info() (Info, error)
}
