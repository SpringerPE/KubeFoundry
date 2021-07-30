package main

import (
	"fmt"
	"io"
	"os"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

type GitRepo struct {
	path      string
	url       string
	reference string
	memory    bool
	repo      *git.Repository
}

func NewGitRepo(local string, memory bool) (*GitRepo, error) {
	f, err := os.Open(local)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		g := GitRepo{
			path:   local,
			memory: memory,
		}
		return &g, nil
	}
	return nil, err
}

func (g *GitRepo) Clone(remote, reference, string, recursive bool) error {
	ref := plumbing.ReferenceName(reference)
	options := git.CloneOptions{
		URL:  remote,
		Tags: git.AllTags,
	}
	if recursive {
		options.RecurseSubmodules = git.DefaultSubmoduleRecursionDepth
	}
	switch {
	case ref.IsBranch():
		options.ReferenceName = ref
		options.SingleBranch = true
	case ref.IsTag():
		options.ReferenceName = ref
	default:
		return fmt.Errorf("invalid git reference: %s", reference)
	}
	if g.memory {
		// Git objects storer based on memory
		storer := memory.NewStorage()
		fs := osfs.New(g.path)
		// Clones the repository into the worktree (fs) and storer all the .git
		// content into the storer
		repo, err := git.Clone(storer, fs, &options)
	} else {
		repo, err := git.PlainClone(g.path, false, &options)
	}
	if err == nil {
		g.url = remote
		g.reference = ref.String()
		g.repo = repo
	}
	return err
}

func (g *GitRepo) Delete() error {
	// TODO
	return nil
}
