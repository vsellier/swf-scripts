package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	arg "github.com/alexflint/go-arg"
	git "gopkg.in/src-d/go-git.v4"
	plumbing "gopkg.in/src-d/go-git.v4/plumbing"
)

var (
	catalog []Project
	gitUrl  = "git@github.com"
)

var args struct {
	Catalog  string `arg:"positional" help:"Path to the catalog path"`
	Projects string `arg:"positional" help:"List of project to release, use comma between projects, ex: platform-ui,doc-style"`
	WorkDir  string `arg:"env:WORK_DIR"`
}

type Project struct {
	Name                 string  `json:"name"`
	Organization         string  `json:"git_organization"`
	Labels               string  `json:"labels"`
	MavenVersionProperty string  `json:"maven_property_version"`
	Release              Release `json:"release"`
}

type Release struct {
	Branch          string `json:"branch"`
	Version         string `json:"version"`
	CurrentSnapshot string `json:"current_snapshot_version"`
	NextSnapshot    string `json:"next_snapshot_version"`
}

func loadFromJson(b []byte) []Project {
	var p []Project
	err := json.Unmarshal(b, &p)
	if err != nil {
		log.Fatal(err)
	}
	return p
}

func getProjectInCatalog(n string) (Project, error) {
	for _, p := range catalog {
		if n == p.Name {
			return p, nil
		}
	}
	return Project{}, errors.New("No project " + n + " found in the catalog")
}

func getRepository(p Project) (*git.Repository, error) {
	gitUrl := gitUrl + ":" + p.Organization + "/" + p.Name
	dir := args.WorkDir + "/" + p.Organization + "/" + p.Name

	log.WithFields(log.Fields{
		"gitUrl":    gitUrl,
		"directory": dir,
	}).Info("Cloning project...")

	err := os.MkdirAll(args.WorkDir+"/"+p.Organization, os.ModePerm)
	if err != nil {
		return nil, err
	}

	log.Info("Opening repository on ", dir)
	r, err := git.PlainOpen(dir)
	if err != nil {
		log.WithField("error", err).Info("Error open ", dir, ", Try to clean and clone it")
		os.Remove(dir)

		r, err = git.PlainClone(dir, false, &git.CloneOptions{
			URL:               gitUrl,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			Progress:          os.Stdout,
		})
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

func checkoutAndResetBranch(r *git.Repository, b string) {
	log.Info("checkout and reset " + b)
	w, err := r.Worktree()
	if err != nil {
		log.Fatal(err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(fmt.Sprintf("ref/heads/%s", b)),
		Force:  true,
	})
	if err != nil {
		log.Fatal(err)
	}

}

func createStableBranch(p Project) error {
	r, err := getRepository(p)
	if err != nil {
		return err
	}
	stableVersion := strings.NewReplacer("-SNAPSHOT", "").Replace(p.Release.CurrentSnapshot)
	stableBranch := "stable/" + stableVersion
	originBranch := p.Release.Branch
	log.WithFields(log.Fields{
		"project":      p.Name,
		"originBranch": originBranch,
		"stableBranch": stableBranch,
	}).Info("Creating stable branch")
	checkoutAndResetBranch(r, originBranch)
	// checkoutAndResetBRanch(r, stableBranch, false)
	return nil
}

func init() {
	args.WorkDir = "work"
}

func main() {
	arg.MustParse(&args)

	log.Info("Loading catalog file", args.Catalog)

	b, err := ioutil.ReadFile(args.Catalog) // just pass the file name
	if err != nil {
		log.Fatal("Error reading catalog file ", args.Catalog, err)
	}
	log.Info("Catalog loaded")
	log.Info("Reading json")
	catalog = loadFromJson(b)
	log.Info("Json read")

	projects := strings.Split(args.Projects, ",")

	log.Info(projects)
	for _, p := range projects {
		log.Warn("Creating stable branch for project ", p)
		cp, err := getProjectInCatalog(p)
		if err != nil {
			log.Fatal(err)
		}
		log.WithFields(log.Fields{
			"name":            cp.Name,
			"orga":            cp.Organization,
			"current_version": cp.Release.CurrentSnapshot,
			"next_version":    cp.Release.NextSnapshot,
		}).Info("project found")

		err = createStableBranch(cp)
		if err != nil {
			log.Fatal(err)
		}

	}
}
