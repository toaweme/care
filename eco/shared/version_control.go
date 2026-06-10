package shared

import (
	"context"
	"fmt"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/internal/devops/git"
)

type versionControl struct{ mend.BaseCheck }

var _ mend.VersionControl = (*versionControl)(nil)

// NewVersionControl reports the working-tree and upstream-sync state of a repo. It
// is language-agnostic (it shells out to git via internal/devops/git) and applies
// to any repository.
func NewVersionControl() mend.VersionControl {
	git := mend.NewTool(mend.ToolSpec{Name: "git", Installer: mend.InstallerBuiltin})
	return &versionControl{mend.NewBaseCheck("git", git)}
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
		rf = append(rf, mend.RepoFile{Status: file.StatusString(), Name: file.Name})
	}
	report := mend.VCReport{Files: rf, HasUpstream: sync.HasUpstream}
	if sync.HasUpstream {
		report.Ahead = sync.Ahead
		report.Behind = sync.Behind
	}
	return mend.Fail(report)
}
