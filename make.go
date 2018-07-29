package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	dh "github.com/foxboron/dh-make-golang/src"
	"golang.org/x/tools/go/vcs"
)

var GithubLicenseToDistroLicense = map[string]string{
	"agpl-3.0":     "AGPL",
	"apache-2.0":   "Apache",
	"artistic-2.0": "Artistic2.0",
	"cc0-1.0":      "CC0-1.0",
	"epl-1.0":      "EPL",
	"gpl-2.0":      "GPL",
	"gpl-3.0":      "GPL3",
	"lgpl-2.1":     "LGPL",
	"lgpl-3.0":     "LGPL3",
	"mpl-2.0":      "MPL2",
}

var ArchSpecialLincenses = map[string]string{
	"bsd-2-clause": "BSD",
	"bsd-3-clause": "BSD",
	"isc":          "ISC",
	"unlicense":    "Unlicense",
	"mit":          "MIT",
}

type Pkgbuild struct {
	Pkgname            string
	Pkgver             string
	Pkgrel             string
	Description        string
	Lisence            string
	Url                string
	NonStandardLisence bool
	Depends            []string
	Revision           string
	Repository         string
	DirectoryName      string
}

func GetLicense(gopkg string) (string, bool, error) {
	githubLicense, err := dh.GetLicenseForGopkg(gopkg)
	if err != nil {
		return "TODO", false, err
	}

	if license, ok := GithubLicenseToDistroLicense[githubLicense]; ok {
		return license, false, nil
	}

	if license, ok := ArchSpecialLincenses[githubLicense]; ok {
		return license, true, nil
	}
	return "TODO", false, fmt.Errorf("Could not determine lincense")
}

func GetRevision(gitdir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = gitdir
	revision, err := cmd.Output()
	if err != nil {
		return "", err
	}
	rev := string(revision)
	return strings.TrimSpace(rev), nil
}

func CreatePackage(directory, gopkg, gopath, gitRevision, pkgType string) []string {
	// Ensure the specified argument is a Go package import path.
	rr, err := vcs.RepoRootForImportPath(gopkg, false)
	if err != nil {
		log.Fatalf("Verifying arguments: %v — did you specify a Go package import path?", err)
	}
	if gopkg != rr.Root {
		log.Printf("Continuing with repository root %q instead of specified import path %q", rr.Root, gopkg)
		gopkg = rr.Root
	}

	if strings.ToLower(gopkg) != gopkg {
		log.Printf("WARNING: Go package names are case-sensitive. Did you really mean %q instead of %q?\n",
			gopkg, strings.ToLower(gopkg))
	}

	u, err := dh.NewUpstreamSource(gopath, gopkg, gitRevision)
	if err != nil {
		log.Printf("Could not create a tarball of the upstream source: %v\n", err)
		return []string{}
	}

	if strings.TrimSpace(pkgType) == "" {
		if u.FirstMain != "" {
			log.Printf("Assuming you are packaging a program (because %q defines a main package), use -type to override\n", u.FirstMain)
			pkgType = "program"
		} else {
			pkgType = "library"
		}
	}
	var pkgname string
	allowUnknownHoster := true
	pkgname = dh.NameFromGopkg(gopkg, pkgType, allowUnknownHoster)

	if _, err := os.Stat(path.Join(directory, pkgname)); err == nil {
		log.Printf("Output directory %q already exists, quitting\n", path.Join(directory, pkgname))
		return u.RepoDeps
	}

	dependencies := make([]string, 0, len(u.RepoDeps))
	for _, dep := range u.RepoDeps {
		dependencies = append(dependencies, dh.NameFromGopkg(dep, "library", allowUnknownHoster))
	}

	license, special, err := GetLicense(gopkg)
	if err != nil {
		license = "TODO"
		log.Printf("%s missing lisence. Marked as TODO", pkgname)
	}

	desc, err := dh.GetDescriptionForGopkg(gopkg)
	if err != nil {
		desc = "TODO"
		log.Printf("%s missing description. Marked as TODO", pkgname)
	}

	rev, err := GetRevision(u.RepoDir)
	if err != nil {
		rev = "TODO"
		log.Printf("%s missing revision. Marked as TODO", pkgname)
	}

	pkgbuild := &Pkgbuild{
		Pkgname:            pkgname,
		Pkgver:             u.Version,
		Description:        desc,
		Lisence:            license,
		Url:                dh.GetHomepageForGopkg(gopkg),
		NonStandardLisence: special,
		Depends:            dependencies,
		Revision:           rev,
		Repository:         gopkg,
		DirectoryName:      path.Base(gopkg),
	}
	os.Mkdir(path.Join(directory, pkgname), 0755)
	f, err := os.Create(path.Join(directory, pkgname, "PKGBUILD"))
	if err != nil {
		log.Printf("Couldn't create file: %s", err)
	}
	tpl := template.Must(template.New("main").Funcs(template.FuncMap{"StringsJoin": strings.Join}).ParseGlob("./templates/*.template"))
	err = tpl.ExecuteTemplate(f, fmt.Sprintf("pkgbuild-%s.template", pkgType), pkgbuild)
	if err != nil {
		log.Printf("Couldn't write templtae: %s", err)
		return u.RepoDeps
	}
	log.Printf("Created package %s in %s", pkgname, path.Join(directory, pkgname))
	return u.RepoDeps
}

func execMake(args []string) {
	dh.InitGithub()

	fs := flag.NewFlagSet("make", flag.ExitOnError)

	var gitRevision string
	fs.StringVar(&gitRevision,
		"git_revision",
		"",
		"git revision (see gitrevisions(7)) of the specified Go package to check out, defaulting to the default behavior of git clone. Useful in case you do not want to package e.g. current HEAD.")

	var pkgType string
	fs.StringVar(&pkgType,
		"type",
		"",
		"One of \"library\" or \"program\"")

	packageDependencies := fs.Bool(
		"deps",
		false,
		"Package dependencies of the library/program")

	err := fs.Parse(args)
	if err != nil {
		log.Fatal(err)
	}

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	gitRevision = strings.TrimSpace(gitRevision)
	gopkg := fs.Arg(0)
	os.Mkdir("./packages", 0755)
	// gopath, err := ioutil.TempDir("/var/tmp", "dh-make-golang")
	// if err != nil {
	// 	log.Fatalf("Could not create a tmp directory")
	// }
	gopath := "/var/tmp/dh-make-golang"
	os.Mkdir(gopath, 0755)
	defer os.RemoveAll(gopath)

	dependencies := CreatePackage("./packages", gopkg, gopath, gitRevision, pkgType)
	if !*packageDependencies {
		log.Println("Dependencies:")
		for _, k := range dependencies {
			log.Printf("\t%s", k)
		}
		os.Exit(0)
	}

	var dep string
	for {
		dep, dependencies = dependencies[0], dependencies[1:]
		newDeps := CreatePackage("./packages", dep, gopath, "", "library")
		for _, k := range newDeps {
			new := true
			for _, k2 := range dependencies {
				if k == k2 {
					new = false
					break
				}
			}
			if new {
				dependencies = append(dependencies, k)
			}
		}
	}
}
