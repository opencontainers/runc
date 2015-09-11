package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// DefaultRules are the standard validation to perform on git commits
var DefaultRules = []ValidateRule{
	func(c CommitEntry) (vr ValidateResult) {
		vr.CommitEntry = c
		if len(strings.Split(c["parent"], " ")) > 1 {
			vr.Pass = true
			vr.Msg = "merge commits do not require DCO"
			return vr
		}

		hasValid := false
		for _, line := range strings.Split(c["body"], "\n") {
			if validDCO.MatchString(line) {
				hasValid = true
			}
		}
		if !hasValid {
			vr.Pass = false
			vr.Msg = "does not have a valid DCO"
		} else {
			vr.Pass = true
			vr.Msg = "has a valid DCO"
		}

		return vr
	},
	// TODO add something for the cleanliness of the c.Subject
}

var (
	flVerbose     = flag.Bool("v", false, "verbose")
	flCommitRange = flag.String("range", "", "use this commit range instead")

	validDCO = regexp.MustCompile(`^Signed-off-by: ([^<]+) <([^<>@]+@[^<>]+)>$`)
)

func main() {
	flag.Parse()

	var commitrange string
	if *flCommitRange != "" {
		commitrange = *flCommitRange
	} else {
		var err error
		commitrange, err = GitFetchHeadCommit()
		if err != nil {
			log.Fatal(err)
		}
	}

	c, err := GitCommits(commitrange)
	if err != nil {
		log.Fatal(err)
	}

	results := ValidateResults{}
	for _, commit := range c {
		fmt.Printf(" * %s %s ... ", commit["abbreviated_commit"], commit["subject"])
		vr := ValidateCommit(commit, DefaultRules)
		results = append(results, vr...)
		if _, fail := vr.PassFail(); fail == 0 {
			fmt.Println("PASS")
			if *flVerbose {
				for _, r := range vr {
					if r.Pass {
						fmt.Printf("  - %s\n", r.Msg)
					}
				}
			}
		} else {
			fmt.Println("FAIL")
			// default, only print out failed validations
			for _, r := range vr {
				if !r.Pass {
					fmt.Printf("  - %s\n", r.Msg)
				}
			}
		}
	}
	_, fail := results.PassFail()
	if fail > 0 {
		fmt.Printf("%d issues to fix\n", fail)
		os.Exit(1)
	}
}

// ValidateRule will operate over a provided CommitEntry, and return a result.
type ValidateRule func(CommitEntry) ValidateResult

// ValidateCommit processes the given rules on the provided commit, and returns the result set.
func ValidateCommit(c CommitEntry, rules []ValidateRule) ValidateResults {
	results := ValidateResults{}
	for _, r := range rules {
		results = append(results, r(c))
	}
	return results
}

// ValidateResult is the result for a single validation of a commit.
type ValidateResult struct {
	CommitEntry CommitEntry
	Pass        bool
	Msg         string
}

// ValidateResults is a set of results. This is type makes it easy for the following function.
type ValidateResults []ValidateResult

// PassFail gives a quick over/under of passes and failures of the results in this set
func (vr ValidateResults) PassFail() (pass int, fail int) {
	for _, res := range vr {
		if res.Pass {
			pass++
		} else {
			fail++
		}
	}
	return pass, fail
}

// CommitEntry represents a single commit's information from `git`
type CommitEntry map[string]string

var (
	prettyFormat         = `--pretty=format:`
	formatSubject        = `%s`
	formatBody           = `%b`
	formatCommit         = `%H`
	formatAuthorName     = `%aN`
	formatAuthorEmail    = `%aE`
	formatCommitterName  = `%cN`
	formatCommitterEmail = `%cE`
	formatSigner         = `%GS`
	formatCommitNotes    = `%N`
	formatMap            = `{"commit": "%H", "abbreviated_commit": "%h", "tree": "%T", "abbreviated_tree": "%t", "parent": "%P", "abbreviated_parent": "%p", "refs": "%D", "encoding": "%e", "sanitized_subject_line": "%f", "verification_flag": "%G?", "signer_key": "%GK", "author_date": "%aD" , "committer_date": "%cD" }`
)

// GitLogCommit assembles the full information on a commit from its commit hash
func GitLogCommit(commit string) (*CommitEntry, error) {
	buf := bytes.NewBuffer([]byte{})
	cmd := exec.Command("git", "log", "-1", prettyFormat+formatMap, commit)
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Println(strings.Join(cmd.Args, " "))
		return nil, err
	}
	c := CommitEntry{}
	output := buf.Bytes()
	if err := json.Unmarshal(output, &c); err != nil {
		fmt.Println(string(output))
		return nil, err
	}

	// any user provided fields can't be sanitized for the mock-json marshal above
	for k, v := range map[string]string{
		"subject":         formatSubject,
		"body":            formatBody,
		"author_name":     formatAuthorName,
		"author_email":    formatAuthorEmail,
		"committer_name":  formatCommitterName,
		"committer_email": formatCommitterEmail,
		"commit_notes":    formatCommitNotes,
		"signer":          formatSigner,
	} {
		output, err := exec.Command("git", "log", "-1", prettyFormat+v, commit).Output()
		if err != nil {
			return nil, err
		}
		c[k] = strings.TrimSpace(string(output))
	}

	return &c, nil
}

// GitCommits returns a set of commits.
// If commitrange is a git still range 12345...54321, then it will be isolated set of commits.
// If commitrange is a single commit, all ancestor commits up through the hash provided.
func GitCommits(commitrange string) ([]CommitEntry, error) {
	output, err := exec.Command("git", "log", prettyFormat+formatCommit, commitrange).Output()
	if err != nil {
		return nil, err
	}
	commitHashes := strings.Split(strings.TrimSpace(string(output)), "\n")
	commits := make([]CommitEntry, len(commitHashes))
	for i, commitHash := range commitHashes {
		c, err := GitLogCommit(commitHash)
		if err != nil {
			return commits, err
		}
		commits[i] = *c
	}
	return commits, nil
}

// GitFetchHeadCommit returns the hash of FETCH_HEAD
func GitFetchHeadCommit() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--verify", "FETCH_HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GitHeadCommit returns the hash of HEAD
func GitHeadCommit() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--verify", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
