package shared

import (
	"context"
	"fmt"
	"sort"

	"github.com/toaweme/care"
	"github.com/toaweme/care/internal/devops/git"
)

type versionControl struct{ care.BaseCheck }

var _ care.VersionControl = (*versionControl)(nil)

// NewVersionControl reports the working-tree and upstream-sync state of a repo. It
// is language-agnostic (it shells out to git via internal/devops/git) and applies
// to any repository.
func NewVersionControl() care.VersionControl {
	gitTool := care.NewTool(care.ToolSpec{Name: "git", Installer: care.InstallerBuiltin})
	return &versionControl{care.NewBaseCheck("git", gitTool)}
}

func (f *versionControl) Applies(string) bool { return true }

func (f *versionControl) Run(_ context.Context, dir string, _ care.RunOptions) care.Output[care.VCReport] {
	r := git.NewRepository(dir)
	files, err := r.Status()
	if err != nil {
		return care.Errored[care.VCReport]("git failed", fmt.Errorf("failed to read git status: %w", err))
	}
	sync, err := r.SyncStatus()
	if err != nil {
		return care.Errored[care.VCReport]("git failed", fmt.Errorf("failed to read sync status: %w", err))
	}
	if len(files) == 0 && sync.InSync() {
		return care.Pass(care.VCReport{HasUpstream: sync.HasUpstream})
	}
	rf := make([]care.RepoFile, 0, len(files))
	for _, file := range files {
		entry := care.RepoFile{Status: file.StatusString(), Name: file.Name, Path: file.Path, Added: file.Added, Deleted: file.Deleted}
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
	report := care.VCReport{Files: rf, HasUpstream: sync.HasUpstream}
	if sync.HasUpstream {
		report.Ahead = sync.Ahead
		report.Behind = sync.Behind
	}
	return care.Fail(report)
}
