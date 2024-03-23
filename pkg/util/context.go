package util

import "context"

type VersionKey struct{}

func SetVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, VersionKey{}, version)
}

func GetVersion(ctx context.Context) string {
	val := ctx.Value(VersionKey{})
	version, ok := val.(string)
	if !ok {
		return "dev"
	}
	return version
}

type GitCommitKey struct{}

func SetGitCommit(ctx context.Context, commit string) context.Context {
	return context.WithValue(ctx, GitCommitKey{}, commit)
}

func GetGitCommit(ctx context.Context) string {
	val := ctx.Value(GitCommitKey{})
	commit, ok := val.(string)
	if !ok {
		return "dev"
	}
	return commit
}
