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

type filterAccum struct {
	include  bool
	set      func(*compiler.IncludeSpec, string)
	tagGlobs *[]compiler.IncludeSpec
}

func (fa *filterAccum) String() string {
	var sb strings.Builder
	if fa.tagGlobs != nil {
		for i, ta := range *fa.tagGlobs {
			if i != 0 {
				sb.WriteByte((','))
			}
			if ta.Include {
				sb.WriteByte('+')
			} else {
				sb.WriteByte('-')
			}
			sb.WriteString(ta.TagGlob)
		}
	}
	return sb.String()
}

func (fa *filterAccum) Set(s string) error {
	var spec compiler.IncludeSpec
	spec.Include = fa.include
	fa.set(&spec, s)
	*fa.tagGlobs = append(*fa.tagGlobs, spec)
	return nil
}

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
	inputFiles := &inputAccum{}
	flag.Var(inputFiles, "input", "input file (default stdin)")
	output := flag.String("output", "", "output file (default stdout)")
	outputPrefix := flag.String("output-prefix", "", `add a prefix to the output (e.g. "export ROUTES=")`)
	includeSpecs := make([]compiler.IncludeSpec, 0)
	includeTags := &filterAccum{true, func(is *compiler.IncludeSpec, val string) { is.TagGlob = val }, &includeSpecs}
	excludeTags := &filterAccum{false, func(is *compiler.IncludeSpec, val string) { is.TagGlob = val }, &includeSpecs}
	includeMethods := &filterAccum{true, func(is *compiler.IncludeSpec, val string) { is.Method = val }, &includeSpecs}
	excludeMethods := &filterAccum{false, func(is *compiler.IncludeSpec, val string) { is.Method = val }, &includeSpecs}
	flag.Var(includeTags, "include-tags", "include routes with these tags (wildcard pattern)")
	flag.Var(excludeTags, "exclude-tags", "exclude routes with these tags (wildcard pattern)")
	flag.Var(includeMethods, "include-method", "include routes with this method")
	flag.Var(excludeMethods, "exclude-method", "exclude routes with this method")
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

	var filenames []string
	if len(inputFiles.filenames) == 0 {
		filenames = []string{""} // indicates stdin
	} else {
		filenames = inputFiles.filenames
	}

	os.Exit(run(runParams{
		version:        *version,
		inputFiles:     filenames,
		output:         *output,
		outputPrefix:   *outputPrefix,
		verbose:        *verbose,
		allowUpperCase: *allowUpperCase,
		withReader:     withReader,
		withWriter:     withWriter,
		fprintf:        fmt.Fprintf,
		nameSeparator:  *nameSeparator}))
}

type runParams struct {
	version        bool
	inputFiles     []string
	output         string
	outputPrefix   string
	specs          []compiler.IncludeSpec
	verbose        bool
	allowUpperCase bool
	withReader     func(string, func(io.Reader)) error
	withWriter     func(string, func(io.Writer)) error
	fprintf        func(w io.Writer, format string, a ...interface{}) (int, error)
	nameSeparator  string
}

func run(params runParams) int {
	var exitCode int

	if params.version {
		bi, ok := debug.ReadBuildInfo()
		if !ok || bi.Main.Version == "" {
			_, _ = params.fprintf(os.Stdout, "claney version unknown\n")
			return 0
		}
		_, _ = params.fprintf(os.Stdout, "claney %+v\n", bi.Main.Version)
		return 0
	}

	err := withReaders([]io.Reader{}, params.inputFiles, params.withReader, func(inputReaders []io.Reader) {
		exitCode = runHelper(params, inputReaders)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	return exitCode
}

func runHelper(params runParams, inputReaders []io.Reader) int {
	casePolicy := compiler.DisallowUpperCase
	if params.allowUpperCase {
		casePolicy = compiler.AllowUpperCase
	}

	entries, errors := compiler.ParseRouteFiles(params.inputFiles, inputReaders, casePolicy)

	if len(errors) > 0 {
		sortRouteErrors(errors)
		for _, e := range errors {
			_, _ = params.fprintf(os.Stderr, "%v\n", e)
		}
		return 1
	}

	metadataOut := os.Stdout
	metadataOutDescription := params.output
	if params.output == "" {
		metadataOut = os.Stderr
		metadataOutDescription = "stdout"
	}

	routes, errors := compiler.ProcessRouteFile(entries, params.inputFiles, params.nameSeparator, func(rwps []compiler.RouteWithParents) {
		if params.verbose {
			_, _ = params.fprintf(metadataOut, "WARNING:\n")
			_, _ = params.fprintf(metadataOut, "  Group of %v routes that must be checked pairwise for overlaps.\n", len(rwps))
			_, _ = params.fprintf(metadataOut, "  This occurs if the routes lack a unique constant prefix or suffix.\n")
			_, _ = params.fprintf(metadataOut, "  Pairwise overlap checks are slow.\n")
			_, _ = params.fprintf(metadataOut, "  Routes in group:\n")

			sorted := make([]*compiler.RouteInfo, len(rwps))
			for i := range rwps {
				sorted[i] = rwps[i].Route
			}
			sort.Slice(sorted, func(i, j int) bool {
				if sorted[i].Filename == sorted[j].Filename {
					return sorted[i].Line < sorted[j].Line
				}
				return sorted[i].Filename < sorted[j].Filename
			})
			for _, r := range sorted {
				_, _ = params.fprintf(metadataOut, "    %v line %v %v: %v\n", r.Filename, r.Line, r.Name)
			}
		}
	})

	if len(errors) > 0 {
		sortRouteErrors(errors)
		for _, e := range errors {
			_, _ = params.fprintf(os.Stderr, "%v\n", e)
		}
		return 1
	}

	routeRegexps := compiler.GetRouteRegexps(routes)
	json, nRoutes := compiler.RouteRegexpsToJSON(&routeRegexps, params.specs)

	retCode := 0

	err := params.withWriter(params.output, func(of io.Writer) {
		_, _ = io.WriteString(of, params.outputPrefix)

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
		_, _ = params.fprintf(metadataOut, "%v %v written to %v\n", nRoutes, routesString, metadataOutDescription)

		retCode = 0
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	return retCode
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
