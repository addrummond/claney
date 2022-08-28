package main

import (
	"flag"
	"fmt"
	"io"
	"os"
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
	verbose := flag.Bool("verbose", false, "print diagnostic information")
	nameSeparator := flag.String("name-separator", "", "name separator (default \"/\")")
	inputFiles := &inputAccum{}
	flag.Var(inputFiles, "input", "input file (default stdin)")
	output := flag.String("output", "", "output file (default stdout)")
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

	if *nameSeparator == "" {
		*nameSeparator = "/"
	}

	var filenames []string
	if len(inputFiles.filenames) == 0 {
		filenames = []string{""} // indicates stdin
	} else {
		filenames = inputFiles.filenames
	}

	os.Exit(run(filenames, *output, includeSpecs, *verbose, withReader, withWriter, fmt.Fprintf, *nameSeparator))
}

func run(inputFiles []string, output string, specs []compiler.IncludeSpec, verbose bool, withReader func(string, func(io.Reader)) error, withWriter func(string, func(io.Writer)) error, fprintf func(w io.Writer, format string, a ...interface{}) (int, error), nameSeparator string) int {
	var exitCode int

	withReaders([]io.Reader{}, inputFiles, withReader, func(inputReaders []io.Reader) {
		exitCode = runHelper(inputFiles, inputReaders, output, specs, verbose, withWriter, fprintf, nameSeparator)
	})

	return exitCode
}

func runHelper(inputFiles []string, inputReaders []io.Reader, output string, specs []compiler.IncludeSpec, verbose bool, withWriter func(string, func(io.Writer)) error, fprintf func(w io.Writer, format string, a ...interface{}) (int, error), nameSeparator string) int {
	entries, errors := compiler.ParseRouteFiles(inputFiles, inputReaders)

	if len(errors) > 0 {
		sortRouteErrors(errors)
		for _, e := range errors {
			fprintf(os.Stderr, "%v\n", e)
		}
		return 1
	}

	metadataOut := os.Stdout
	metadataOutDescription := output
	if output == "" {
		metadataOut = os.Stderr
		metadataOutDescription = "stdout"
	}

	routes, errors := compiler.ProcessRouteFile(entries, inputFiles, nameSeparator, func(rwps []compiler.RouteWithParents) {
		if verbose {
			fprintf(metadataOut, "WARNING:\n")
			fprintf(metadataOut, "  Group of %v routes that must be checked pairwise for overlaps.\n", len(rwps))
			fprintf(metadataOut, "  This occurs if the routes lack a unique constant prefix or suffix.\n")
			fprintf(metadataOut, "  Pairwise overlap checks are slow.\n")
			fprintf(metadataOut, "  Routes in group:\n")

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
				fprintf(metadataOut, "    %v line %v: %v\n", r.Filename, r.Line, r.Name)
			}
		}
	})

	if len(errors) > 0 {
		sortRouteErrors(errors)
		for _, e := range errors {
			fprintf(os.Stderr, "%v\n", e)
		}
		return 1
	}

	routeRegexps := compiler.GetRouteRegexps(routes)
	json, nRoutes := compiler.RouteRegexpsToJSON(&routeRegexps, specs)

	retCode := 0

	err := withWriter(output, func(of io.Writer) {
		_, err := of.Write(json)
		if err != nil {
			fprintf(os.Stderr, "%v\n", err)
			retCode = 1
			return
		}

		routesString := "routes"
		if nRoutes == 1 {
			routesString = "route"
		}

		if output == "" {
			fprintf(metadataOut, "\n")
		}
		fprintf(metadataOut, "%v %v written to %v\n", nRoutes, routesString, metadataOutDescription)

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
