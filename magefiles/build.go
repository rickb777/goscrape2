// See https://magefile.org/

//go:build mage

// Build steps for goscrape2:
package main

import (
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"os"
	"strings"
	"time"
)

const GOLANGCI_VERSION = "v1.60.3"

var (
	date    = time.Now().Format("2006-01-02")
	ldFlags = fmt.Sprintf(`-s -X main.version=%s -X main.date=%s`, gitDescribe(), date)
)

var Default = Build

func Build() error {
	mg.Deps(Test)

	if err := sh.RunV("go", "vet", "./..."); err != nil {
		return err
	}
	if err := sh.RunV("go", "build", "-o", "goscrape2", "-ldflags", ldFlags, "."); err != nil {
		return err
	}
	return nil
}

// install all binaries
func Install() error {
	return sh.RunV("go", "install", "-buildvcs=false", "-ldflags", ldFlags, ".")
}

// run tests
func Test() error {
	sh.Rm("goscrape2")
	return sh.RunV("go", "test", "-timeout", "10s", "-race", "./...")
}

// run unit tests and create test coverage
func TestCoverage() error {
	return sh.RunV("go", "test", "-timeout", "10s", "./...", "-coverprofile", ".testCoverage", "-covermode=atomic", "-coverpkg=./...")
}

// run unit tests and show test coverage in browser
func TestCoverageWeb() error {
	lines, err := sh.Output("go", "tool", "cover", "-func", ".testCoverage")
	if e := sh.ExitStatus(err); e != 0 {
		os.Exit(e)
	}
	for _, line := range strings.Split(lines, "\n") {
		if strings.Contains(line, "total") {
			words := strings.Split(line, " ")
			if len(words) > 2 {
				fmt.Println("Total coverage:", words[2])
			} else {
				fmt.Println(line)
			}
		}
	}
	return sh.RunV("go", "tool", "cover", "-html=.testCoverage")
}

// build release binaries from current git state as snapshot
func ReleaseSnapshot() error {
	return sh.RunV("goreleaser", "release", "--snapshot", "--clean")
}

func gitDescribe() string {
	s, err := sh.Output("git", "describe", "--tags", "--always", "--dirty")
	if err != nil {
		return "dev"
	}
	return s
}
