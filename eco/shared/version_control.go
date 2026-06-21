package shared

import (
	"context"
	"fmt"
	"sort"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/internal/devops/git"
)

type versionControl struct{ mend.BaseCheck }

var _ mend.VersionControl = (*versionControl)(nil)

// NewVersionControl reports the working-tree and upstream-sync state of a repo. It
// is language-agnostic (it shells out to git via internal/devops/git) and applies
// to any repository.
func NewVersionControl() mend.VersionControl {
	gitTool := mend.NewTool(mend.ToolSpec{Name: "git", Installer: mend.InstallerBuiltin})
	return &versionControl{mend.NewBaseCheck("git", gitTool)}
}

func (f *versionControl) Applies(string) bool { return true }

func (f *versionControl) Run(_ context.Context, dir string, _ mend.RunOptions) mend.Output[mend.VCReport] {
	r := git.NewRepository(dir)
	files, err := r.Status()
	if err != nil {
		return mend.Errored[mend.VCReport]("git failed", fmt.Errorf("failed to read git status: %w", err))
	}
	sync, err := r.SyncStatus()
	if err != nil {
		return mend.Errored[mend.VCReport]("git failed", fmt.Errorf("failed to read sync status: %w", err))
	}
	if len(files) == 0 && sync.InSync() {
		return mend.Pass(mend.VCReport{HasUpstream: sync.HasUpstream})
	}
	rf := make([]mend.RepoFile, 0, len(files))
	for _, file := range files {
		entry := mend.RepoFile{Status: file.StatusString(), Name: file.Name, Path: file.Path, Added: file.Added, Deleted: file.Deleted}
		if !file.ModTime.IsZero() {
			t := file.ModTime
			entry.TouchedAt = &t
		}
		rf = append(rf, entry)
	}
	// order most-recently-touched first so the report (terminal and JSON) reads as a
	// worklog; files with no mtime (deleted) sort last.
	sort.SliceStable(rf, func(i, j int) bool {
		ti, tj := rf[i].TouchedAt, rf[j].TouchedAt
		if ti == nil || tj == nil {
			return ti != nil && tj == nil
		}
		return ti.After(*tj)
	})
	report := mend.VCReport{Files: rf, HasUpstream: sync.HasUpstream}
	if sync.HasUpstream {
		report.Ahead = sync.Ahead
		report.Behind = sync.Behind
	}
	return mend.Fail(report)
}
