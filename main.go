package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/addrummond/claney/compiler"
)

var version string // to be overriden by goreleaser

type inputAccum struct {
	filenames []string
}

func (ia *inputAccum) String() string {
	return strings.Join(ia.filenames, ", ")
}

func (ia *inputAccum) Set(s string) error {
	ia.filenames = append(ia.filenames, s)
	return nil
}

func main() {
	version := flag.Bool("version", false, "show version information")
	verbose := flag.Bool("verbose", false, "print diagnostic information")
	allowUpperCase := flag.Bool("allow-upper-case", false, "allow upper case characters in routes")
	nameSeparator := flag.String("name-separator", "", "name separator (default \"/\")")
	fancyInputFiles := &inputAccum{}
	jsonInputFiles := &inputAccum{}
	flag.Var(fancyInputFiles, "input", "input file (default stdin)")
	flag.Var(jsonInputFiles, "json-input", "JSON input file")
	jsonStdin := flag.Bool("json-stdin", false, "interpret stdin as JSON (as with -json-input)")
	output := flag.String("output", "", "output file (default stdout)")
	filter := flag.String("filter", "", "include only routes with tags that match the given expression")
	flag.Parse()

	// Claney doesn't take any bare arguments, so print the usage message and exit
	// if any are passed.
	if flag.Arg(0) != "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *nameSeparator == "" {
		*nameSeparator = "/"
	}

	var fancyFilenames []string
	var jsonFilenames []string
	if len(fancyInputFiles.filenames) == 0 && len(jsonInputFiles.filenames) == 0 {
		if *jsonStdin {
			jsonFilenames = []string{""} // indicates stdin
		} else {
			fancyFilenames = []string{""} // indicates stdin
		}
	} else {
		fancyFilenames = fancyInputFiles.filenames
		jsonFilenames = jsonInputFiles.filenames
	}

	os.Exit(run(runParams{
		version:         *version,
		fancyInputFiles: fancyFilenames,
		jsonInputFiles:  jsonFilenames,
		output:          *output,
		filter:          *filter,
		verbose:         *verbose,
		allowUpperCase:  *allowUpperCase,
		withReader:      withReader,
		withWriter:      withWriter,
		fprintf:         fmt.Fprintf,
		nameSeparator:   *nameSeparator}))
}

type runParams struct {
	version         bool
	fancyInputFiles []string
	jsonInputFiles  []string
	output          string
	filter          string
	verbose         bool
	allowUpperCase  bool
	withReader      func(string, func(io.Reader)) error
	withWriter      func(string, func(io.Writer)) error
	fprintf         func(w io.Writer, format string, a ...interface{}) (int, error)
	nameSeparator   string
}

func run(params runParams) int {
	var exitCode int

	if params.version {
		if version != "" {
			_, _ = params.fprintf(os.Stdout, "claney %+v\n", version)
			return 0
		}

		bi, ok := debug.ReadBuildInfo()
		if !ok || bi.Main.Version == "" {
			_, _ = params.fprintf(os.Stdout, "claney version unknown\n")
			return 0
		}
		_, _ = params.fprintf(os.Stdout, "claney %+v\n", bi.Main.Version)
		return 0
	}

	err := withReaders([]io.Reader{}, params.fancyInputFiles, params.withReader, func(fancyInputReaders []io.Reader) {
		withReaders([]io.Reader{}, params.jsonInputFiles, params.withReader, func(jsonInputReaders []io.Reader) {
			exitCode = runHelper(params, fancyInputReaders, jsonInputReaders)
		})
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	return exitCode
}

func parseInputFiles(fancyInputFiles []string, jsonInputFiles []string, fancyInputReaders []io.Reader, jsonInputReaders []io.Reader, casePolicy compiler.CasePolicy, nameSeparator string) (routes []compiler.CompiledRoute, errors []compiler.RouteError) {
	jsonStart := len(fancyInputFiles)
	allInputFiles := append(append([]string{}, fancyInputFiles...), jsonInputFiles...)
	allInputReaders := append(append([]io.Reader{}, fancyInputReaders...), jsonInputReaders...)
	var entries [][]compiler.RouteFileEntry
	entries, errors = compiler.ParseRouteFiles(allInputFiles, allInputReaders, jsonStart, casePolicy)
	if len(errors) > 0 {
		return
	}
	routes, errors = compiler.ProcessRouteFiles(entries, allInputFiles, nameSeparator)
	return
}

func runHelper(params runParams, fancyInputReaders []io.Reader, jsonInputReaders []io.Reader) int {
	metadataOut := os.Stdout
	if params.output == "" {
		metadataOut = os.Stderr
	}

	metadataOutDescription := ""
	if params.output != "" {
		metadataOutDescription = " written to " + params.output
	}

	casePolicy := compiler.DisallowUpperCase
	if params.allowUpperCase {
		casePolicy = compiler.AllowUpperCase
	}

	filter, filterErr := compiler.ParseTagExpr(params.filter)
	if filterErr != nil {
		params.fprintf(os.Stderr, "Error parsing value of -filter option:\n%v\n", filterErr)
		return 1
	}

	routes, errors := parseInputFiles(params.fancyInputFiles, params.jsonInputFiles, fancyInputReaders, jsonInputReaders, casePolicy, params.nameSeparator)
	errors = append(errors, compiler.CheckForGroupErrors(routes)...)

	if len(errors) > 0 {
		sortRouteErrors(errors)
		for _, e := range errors {
			if params.verbose && e.Kind == compiler.WarningBigGroup {
				printBigGroupWarning(params, metadataOut, e)
			}
			if params.verbose || e.Kind&compiler.RouteWarning == 0 {
				_, _ = params.fprintf(os.Stderr, "%v\n", e)
			}
		}
		return 1
	}

	routeRegexps := compiler.GetRouteRegexps(routes, filter)
	json, nRoutes := compiler.RouteRegexpsToJSON(&routeRegexps, filter)

	retCode := 0

	err := params.withWriter(params.output, func(of io.Writer) {
		_, err := of.Write(json)
		if err != nil {
			_, _ = params.fprintf(os.Stderr, "%v\n", err)
			retCode = 1
			return
		}

		routesString := "routes"
		if nRoutes == 1 {
			routesString = "route"
		}

		if params.output == "" {
			_, _ = params.fprintf(metadataOut, "\n")
		}
		_, _ = params.fprintf(metadataOut, "%v %v%v\n", nRoutes, routesString, metadataOutDescription)

		retCode = 0
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	return retCode
}

func printBigGroupWarning(params runParams, metadataOut *os.File, err compiler.RouteError) {
	sorted := make([]*compiler.CompiledRoute, len(err.Group))
	for i := range err.Group {
		sorted[i] = err.Group[i].Route
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Info.Filename == sorted[j].Info.Filename {
			return sorted[i].Info.Line < sorted[j].Info.Line
		}
		return sorted[i].Info.Filename < sorted[j].Info.Filename
	})

	_, _ = params.fprintf(metadataOut, "WARNING: Big group\n")
	_, _ = params.fprintf(metadataOut, "  Group of %v routes that must be checked pairwise for overlaps.\n", len(err.Group))
	_, _ = params.fprintf(metadataOut, "  This occurs if the routes lack a unique constant prefix or suffix.\n")
	_, _ = params.fprintf(metadataOut, "  Pairwise overlap checks are slow.\n")
	_, _ = params.fprintf(metadataOut, "  Routes in group:\n")

	for _, r := range sorted {
		_, _ = params.fprintf(metadataOut, "    %v:%v: %v\n", r.Info.Filename, r.Info.Line, r.Info.Name)
	}
}

func withReader(input string, f func(io.Reader)) error {
	if input == "" {
		f(os.Stdin)
		return nil
	}
	inf, err := os.Open(input)
	if err != nil {
		return err
	}
	defer inf.Close()
	f(inf)
	return nil
}

func withWriter(input string, f func(io.Writer)) error {
	if input == "" {
		f(os.Stdout)
		return nil
	}
	inf, err := os.Create(input)
	if err != nil {
		return err
	}
	defer inf.Close()
	f(inf)
	return nil
}

func withReaders(accum []io.Reader, inputFiles []string, withReader func(string, func(io.Reader)) error, f func([]io.Reader)) error {
	if len(inputFiles) == 0 {
		f(accum)
		return nil
	}

	first := inputFiles[0]
	rest := inputFiles[1:]

	var firstErr error
	err := withReader(first, func(r io.Reader) {
		accum = append(accum, r)
		err := withReaders(accum, rest, withReader, f)
		if err != nil {
			firstErr = err
		}
	})
	if err != nil {
		firstErr = err
	}

	return firstErr
}

func sortRouteErrors(res []compiler.RouteError) {
	sort.Slice(res, func(i, j int) bool {
		if len(res[i].Filenames) > 0 && len(res[j].Filenames) > 0 && res[i].Filenames[0] != res[j].Filenames[0] {
			return res[i].Filenames[0] < res[j].Filenames[0]
		}
		if len(res[i].Filenames) == 0 && len(res[j].Filenames) != 0 {
			return true
		}
		if len(res[j].Filenames) == 0 && len(res[i].Filenames) != 0 {
			return false
		}
		if res[i].Line != res[j].Line {
			return res[i].Line < res[j].Line
		}
		return res[i].OtherLine < res[j].OtherLine
	})
}
