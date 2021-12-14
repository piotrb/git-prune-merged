package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/piotrb/go-utils/git"
	"github.com/piotrb/go-utils/utils"

	git "github.com/libgit2/git2go/v33"
)

var force = flag.Bool("f", false, "Force cleanup even based on irregular branches")

func handleError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func gitStatusCount(repo *git.Repository) (int, error) {
	options := git.StatusOptions{Flags: git.StatusOptIncludeUntracked}
	status, err := repo.StatusList(&options)
	if err != nil {
		return 0, err
	}

	count, err := status.EntryCount()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func cleanup(repo *git.Repository, currentBranchName string) {
	if utils.FileExists(".git/MERGE_HEAD") {
		utils.Run("git", "merge", "--abort")
	}
	if utils.FileExists(".git/REBASE_HEAD") {
		utils.Run("git", "rebase", "--abort")
	}
	utils.Run("git", "clean", "-fq")

	currentBranch, err := gitutil.CurrentBranch(repo)
	handleError(err)

	if currentBranch.Name != currentBranchName {
		utils.Run("git", "checkout", currentBranchName)
	}
}

func processBranch(repo *git.Repository, branch gitutil.BranchInfo, candidates []string, currentBranchName string) []string {
	fmt.Printf("[%v] ... ", branch.Name)
	err := utils.RunE("git", "clean", "-fq")
	if err != nil {
		fmt.Print("failed to clean\n")
		fmt.Printf("%o", err)
		return candidates
	}

	_, err = utils.BacktickE("git", "checkout", branch.Name)
	if err != nil {
		fmt.Print("checkout branch failed\n")
		cleanup(repo, currentBranchName)
		return candidates
	}

	_, err = utils.BacktickE("git", "rebase", currentBranchName)
	if err != nil {
		fmt.Print("rebase branch failed\n")
		cleanup(repo, currentBranchName)
		return candidates
	}

	_, err = utils.BacktickE("git", "checkout", currentBranchName)
	if err != nil {
		fmt.Print("checkout current branch failed\n")
		cleanup(repo, currentBranchName)
		return candidates
	}

	_, err = utils.BacktickE("git", "merge", "--no-commit", "--no-ff", branch.Name)
	if err != nil {
		fmt.Print("merge failed\n")
		cleanup(repo, currentBranchName)
		return candidates
	}

	count, err := gitStatusCount(repo)
	if err != nil {
		handleError(err)
	} else {
		if count > 0 {
			fmt.Print("had some extra changes\n")
		} else {
			fmt.Print("merged cleanly\n")
			candidates = append(candidates, branch.Name)
		}
	}

	// cleanup
	cleanup(repo, currentBranchName)

	return candidates
}

func main() {
	flag.Parse()
	candidates := []string{}

	repo, err := gitutil.DiscoverRepo(".")
	handleError(err)

	branches, err := gitutil.AllBranches(repo, git.BranchLocal)
	handleError(err)

	currentBranch, err := gitutil.CurrentBranch(repo)
	handleError(err)

	if currentBranch.Name != "master" && currentBranch.Name != "develop" && !*force {
		fmt.Fprint(os.Stderr, "should be ran from master or develop\n")
		os.Exit(1)
	}

	for _, branch := range branches {
		if branch.Name == "develop" || branch.Name == "master" || branch.Name == "staging" || branch.Name == currentBranch.Name {
			continue
		}

		candidates = processBranch(repo, branch, candidates, currentBranch.Name)
	}

	if len(candidates) == 0 {
		fmt.Print("nothing to clean up\n")
	} else {
		for _, branch := range candidates {
			utils.Run("git", "branch", "-D", branch)
		}
	}
}
