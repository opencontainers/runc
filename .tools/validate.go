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
		hasValid := false
		for _, line := range strings.Split(c.Body, "\n") {
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
		fmt.Printf(" * %s %s ... ", commit.AbbreviatedCommit, commit.Subject)
		vr := ValidateCommit(commit, DefaultRules)
		results = append(results, vr...)
		if _, fail := vr.PassFail(); fail == 0 {
			fmt.Println("PASS")
			if *flVerbose {
				for _, r := range vr {
					if !r.Pass {
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
type CommitEntry struct {
	Commit               string
	AbbreviatedCommit    string `json:"abbreviated_commit"`
	Tree                 string
	AbbreviatedTree      string `json:"abbreviated_tree"`
	Parent               string
	AbbreviatedParent    string `json:"abbreviated_parent"`
	Refs                 string
	Encoding             string
	Subject              string
	SanitizedSubjectLine string `json:"sanitized_subject_line"`
	Body                 string
	CommitNotes          string `json:"commit_notes"`
	VerificationFlag     string `json:"verification_flag"`
	ShortMsg             string
	Signer               string
	SignerKey            string       `json:"signer_key"`
	Author               PersonAction `json:"author,omitempty"`
	Commiter             PersonAction `json:"commiter,omitempty"`
}

// PersonAction is a time and identity of an action on a git commit
type PersonAction struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Date  string `json:"date"` // this could maybe be an actual time.Time
}

var (
	prettyLogSubject       = `--pretty=format:%s`
	prettyLogBody          = `--pretty=format:%b`
	prettyLogCommit        = `--pretty=format:%H`
	prettyLogAuthorName    = `--pretty=format:%aN`
	prettyLogAuthorEmail   = `--pretty=format:%aE`
	prettyLogCommiterName  = `--pretty=format:%cN`
	prettyLogCommiterEmail = `--pretty=format:%cE`
	prettyLogSigner        = `--pretty=format:%GS`
	prettyLogCommitNotes   = `--pretty=format:%N`
	prettyLogFormat        = `--pretty=format:{"commit": "%H", "abbreviated_commit": "%h", "tree": "%T", "abbreviated_tree": "%t", "parent": "%P", "abbreviated_parent": "%p", "refs": "%D", "encoding": "%e", "sanitized_subject_line": "%f", "verification_flag": "%G?", "signer_key": "%GK", "author": { "date": "%aD" }, "commiter": { "date": "%cD" }}`
)

// GitLogCommit assembles the full information on a commit from its commit hash
func GitLogCommit(commit string) (*CommitEntry, error) {
	buf := bytes.NewBuffer([]byte{})
	cmd := exec.Command("git", "log", "-1", prettyLogFormat, commit)
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

	output, err := exec.Command("git", "log", "-1", prettyLogSubject, commit).Output()
	if err != nil {
		return nil, err
	}
	c.Subject = strings.TrimSpace(string(output))

	output, err = exec.Command("git", "log", "-1", prettyLogBody, commit).Output()
	if err != nil {
		return nil, err
	}
	c.Body = strings.TrimSpace(string(output))

	output, err = exec.Command("git", "log", "-1", prettyLogAuthorName, commit).Output()
	if err != nil {
		return nil, err
	}
	c.Author.Name = strings.TrimSpace(string(output))

	output, err = exec.Command("git", "log", "-1", prettyLogAuthorEmail, commit).Output()
	if err != nil {
		return nil, err
	}
	c.Author.Email = strings.TrimSpace(string(output))

	output, err = exec.Command("git", "log", "-1", prettyLogCommiterName, commit).Output()
	if err != nil {
		return nil, err
	}
	c.Commiter.Name = strings.TrimSpace(string(output))

	output, err = exec.Command("git", "log", "-1", prettyLogCommiterEmail, commit).Output()
	if err != nil {
		return nil, err
	}
	c.Commiter.Email = strings.TrimSpace(string(output))

	output, err = exec.Command("git", "log", "-1", prettyLogCommitNotes, commit).Output()
	if err != nil {
		return nil, err
	}
	c.CommitNotes = strings.TrimSpace(string(output))

	output, err = exec.Command("git", "log", "-1", prettyLogSigner, commit).Output()
	if err != nil {
		return nil, err
	}
	c.Signer = strings.TrimSpace(string(output))

	return &c, nil
}

// GitCommits returns a set of commits.
// If commitrange is a git still range 12345...54321, then it will be isolated set of commits.
// If commitrange is a single commit, all ancestor commits up through the hash provided.
func GitCommits(commitrange string) ([]CommitEntry, error) {
	output, err := exec.Command("git", "log", prettyLogCommit, commitrange).Output()
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
	output, err := exec.Command("git", "rev-parse", "--verify", "HEAD").Output()
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
