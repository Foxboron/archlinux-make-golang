package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
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

	gopath, err := ioutil.TempDir("", "dh-make-golang")
	if err != nil {
		log.Fatalf("Could not create a tmp directory")
	}
	defer os.RemoveAll(gopath)

	u, err := dh.NewUpstreamSource(gopath, gopkg, gitRevision)
	if err != nil {
		log.Fatalf("Could not create a tarball of the upstream source: %v\n", err)
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

	if _, err := os.Stat(pkgname); err == nil {
		log.Fatalf("Output directory %q already exists, aborting\n", pkgname)
	}

	dependencies := make([]string, 0, len(u.RepoDeps))
	for _, dep := range u.RepoDeps {
		dependencies = append(dependencies, dh.NameFromGopkg(dep, "library", allowUnknownHoster))
	}

	license, special, err := GetLicense(gopkg)
	if err != nil {
		log.Println(err)
	}

	fmt.Println(gopath)
	fmt.Println(gopkg)
	fmt.Println(pkgname)
	fmt.Println(u.Version)
	fmt.Println(pkgType)
	fmt.Println(dependencies)
	fmt.Println(u.RepoDeps)
	fmt.Println(u.VendorDirs)
	fmt.Println(license)
	fmt.Println(special)

}
