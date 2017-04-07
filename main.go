package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/mgutz/ansi"
	"github.com/spf13/pflag"

	"golang.org/x/tools/cover"
)

const (
	version = "1.0"
)

var (
	ansiReset   = ansi.ColorCode("reset")
	ansiVerbose = ansi.ColorCode("cyan")
	ansiHeader  = ""
	ansiCover   = ""
	ansiUncover = ""

	helpFlag    = pflag.BoolP("help", "h", false, "show this text and exit")
	versionFlag = pflag.BoolP("version", "V", false, "output version information and exit")
	listFlag    = pflag.BoolP("list", "l", false, "list files in the coverage profile")
	doCoverFlag = pflag.BoolP("cover", "c", false, "run tests with the coverage")
	fileFlag    = pflag.StringP("file", "f", "coverage.out", "the coverage profile")

	colorHeaderFlag  = pflag.StringP("color-header", "", "yellow", "set color for header")
	colorCoverFlag   = pflag.StringP("color-cover", "", "green", "set color for covered code")
	colorUncoverFlag = pflag.StringP("color-uncover", "", "red", "set color for not covered code")

	prog = ""
)

func errorf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s: Error: ", prog)
	fmt.Fprintf(os.Stderr, format, v...)
	fmt.Fprintf(os.Stderr, "\n")
}

func fatal(format string, v ...interface{}) {
	errorf(format, v...)
	os.Exit(1)
}

func percentage(profile *cover.Profile) float64 {
	statements := 0
	covered := 0

	for _, block := range profile.Blocks {
		if block.Count > 0 {
			covered += block.NumStmt
		}
		statements += block.NumStmt
	}
	return 100 * float64(covered) / float64(statements)
}

func isMatch(i int, filename string, args []string) bool {
	num := fmt.Sprintf("%d", i)
	for _, arg := range args {
		if arg == num || arg == filename {
			return true
		}
	}
	return false
}

func isExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

func findSource(name string) (string, error) {
	gopaths := os.Getenv("GOPATH")
	if len(gopaths) > 0 {
		arr := filepath.SplitList(gopaths)

		for _, gopath := range arr {
			fpath := filepath.Join(gopath, "src", name)
			if isExists(fpath) {
				return fpath, nil
			}
		}
	}

	goroot := os.Getenv("GOROOT")
	if len(goroot) > 0 {
		fpath := filepath.Join(goroot, "src", name)
		if isExists(fpath) {
			return fpath, nil
		}
	}

	if isExists(name) {
		return name, nil
	}

	return "", os.ErrNotExist
}

func showFile(profile *cover.Profile) error {
	source, err := findSource(profile.FileName)
	if err != nil {
		return err
	}

	lines := make(map[int][]*cover.ProfileBlock)

	for i := range profile.Blocks {
		block := &profile.Blocks[i]

		if block.StartLine != block.EndLine {
			lines[block.StartLine] = append(lines[block.StartLine], block)
		}
		lines[block.EndLine] = append(lines[block.EndLine], block)
	}

	file, err := os.Open(source)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)

	i := 1
	for scanner.Scan() {
		line := scanner.Text()

		keys := []int{}
		marks := make(map[int]string)

		if blocks, ok := lines[i]; ok {
			for _, block := range blocks {
				if i == block.EndLine {
					marks[block.EndCol-1] += ansiReset
					keys = append(keys, block.EndCol-1)
				}
				if i == block.StartLine {
					if block.Count == 0 {
						marks[block.StartCol-1] += ansiUncover
					} else {
						marks[block.StartCol-1] += ansiCover
					}
					keys = append(keys, block.StartCol-1)
				}
			}
		}
		sort.Ints(keys)

		for i := len(keys) - 1; i >= 0; i-- {
			line = line[:keys[i]] + marks[keys[i]] + line[keys[i]:]
		}

		fmt.Printf("%s\n", line)
		i += 1
	}

	return nil
}

func runTestsCoverage() {
	cmd := exec.Command("go", "test", "-coverprofile=coverage.out")
	if err := cmd.Run(); err != nil {
		fatal("%v", err)
	}
}

func showVersion() {
	fmt.Fprintf(os.Stdout, `%s version %s
Written by Alexey Gladkov.

Copyright (C) 2017  Alexey Gladkov <gladkov.alexey@gmail.com>
This is free software; see the source for copying conditions.  There is NO
warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
`,
		prog, version)
}

func main() {
	prog = filepath.Base(os.Args[0])
	pflag.Usage = func() {
		fmt.Fprintf(os.Stdout, `Usage: %[1]s [-l] [-c]
   or: %[1]s [options] (FILE-NUMBER|FILENAME)...

Utility reads the coverage profile and shows annotated source code. You
can create coverage profile, list files in it and then get highlighted
source code.

Options:
%s
Report bugs to author.

`,
			prog, pflag.CommandLine.FlagUsages())
	}

	pflag.Parse()
	args := pflag.Args()

	if *helpFlag {
		pflag.Usage()
		os.Exit(0)
	}

	if *versionFlag {
		showVersion()
		os.Exit(0)
	}

	if len(*colorHeaderFlag) != 0 {
		ansiHeader = ansi.ColorCode(*colorHeaderFlag)
	}

	if len(*colorCoverFlag) != 0 {
		ansiCover = ansi.ColorCode(*colorCoverFlag)
	}

	if len(*colorUncoverFlag) != 0 {
		ansiUncover = ansi.ColorCode(*colorUncoverFlag)
	}

	if *doCoverFlag {
		runTestsCoverage()
		if !*listFlag {
			os.Exit(0)
		}
	}

	if !*listFlag && len(args) == 0 {
		errorf("more arguments required.")
		pflag.Usage()
		os.Exit(2)
	}

	if !isExists(*fileFlag) {
		fatal("file not found: %s", *fileFlag)
	}

	profiles, err := cover.ParseProfiles(*fileFlag)
	if err != nil {
		fatal("%v", err)
	}

	somethingFound := false

	for i, profile := range profiles {
		i += 1

		if *listFlag {
			fmt.Printf("%5d %s%s%s (%.2f%%)\n", i, ansiHeader, profile.FileName, ansiReset, percentage(profile))
			continue
		}

		if !isMatch(i, profile.FileName, args) {
			continue
		}

		somethingFound = true
		fmt.Printf("%scoverage %s (%.2f%%)%s\n", ansiHeader, profile.FileName, percentage(profile), ansiReset)
		if err := showFile(profile); err != nil {
			fatal("%v", err)
		}
	}

	if !somethingFound && !*listFlag {
		errorf("no files were found according to the commandline arguments.")
		os.Exit(2)
	}
}
