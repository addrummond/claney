package main

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/antchfx/jsonquery"
)

const exampleInput = `
root /
  manager /manager\:
	  settings /settings [manager]
		api      /api      [manager,api]
	api     /api
	  getstuff /getstuff [api]
`

// Lower level tests cover most of this, so just some simple tag filtering tests
// here, and checks that error locations are reported correctly.

func TestRunNoTags(t *testing.T) {
	var outb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{""},
		output:          "",
		filter:          "",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockReader(exampleInput),
		withWriter:      mockWriter(&outb),
		fprintf:         dummyFprintf,
		nameSeparator:   "/",
	})
	if exitCode != 0 {
		t.Errorf("Expected 0 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	doc, err := jsonquery.Parse(strings.NewReader(out))
	if err != nil || doc == nil {
		t.Errorf("%v %v", doc, err)
	}
	names := valuesOf[string](jsonquery.Find(doc, "/families/*/members/*/name"))
	if !reflect.DeepEqual(names, []string{"root/api", "root/api/getstuff", "root/manager/settings"}) {
		t.Errorf("Expected all routes to be included in output, got %+v\n", names)
	}
}

func TestRunExcludeAllTags(t *testing.T) {
	var outb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{""},
		output:          "",
		filter:          "man-*&!ap*",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockReader(exampleInput),
		withWriter:      mockWriter(&outb),
		fprintf:         dummyFprintf,
		nameSeparator:   "/",
	})
	if exitCode != 0 {
		t.Errorf("Expected 0 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	doc, err := jsonquery.Parse(strings.NewReader(out))
	if err != nil {
		t.Errorf("%v", err)
	}
	names := valuesOf[string](jsonquery.Find(doc, "/families/*/members/*/name"))
	if len(names) != 0 {
		t.Errorf("Expected no routes to be included in output, got %+v\n", names)
	}
}

func TestRunIncludeOnlySomeTags(t *testing.T) {
	var outb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{""},
		output:          "",
		filter:          "ap*",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockReader(exampleInput),
		withWriter:      mockWriter(&outb),
		fprintf:         dummyFprintf,
		nameSeparator:   "/",
	})
	if exitCode != 0 {
		t.Errorf("Expected 0 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	doc, err := jsonquery.Parse(strings.NewReader(out))
	if err != nil {
		t.Errorf("%v", err)
	}
	names := valuesOf[string](jsonquery.Find(doc, "/families/*/members/*/name"))
	if !reflect.DeepEqual(names, []string{"root/api", "root/api/getstuff"}) {
		t.Errorf("Expected just API routes to be included in output, got %+v\n", names)
	}
}

func TestSyntaxErrorReporting(t *testing.T) {
	const file1 = `
route /foo/bar
notagoodline
another /good/route
	`

	const file2 = `
route /bar/amp
meroute /x/y
notagoodline
another /excellent/good/route
another /good/route

	`

	var outb strings.Builder
	var consoleOutb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{"file1", "file2"},
		output:          "",
		filter:          "",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockMultifileReader(map[string]string{"file1": file1, "file2": file2}),
		withWriter:      mockWriter(&outb),
		fprintf:         getAccumFprintf(&consoleOutb),
		nameSeparator:   "/",
	})
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "file1:3:12: missing route name or missing route pattern\n" +
		"file2:4:12: missing route name or missing route pattern\n"

	if consoleOut != expectedConsoleOut {
		t.Errorf("Did not get expected output, got\n%v\n", consoleOut)
	}
}

func TestUpperCaseReporting(t *testing.T) {
	const file = `
r /
  route /foo/bar/aMP
	`

	var outb strings.Builder
	var consoleOutb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{"file"},
		output:          "",
		filter:          "",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockMultifileReader(map[string]string{"file": file}),
		withWriter:      mockWriter(&outb),
		fprintf:         getAccumFprintf(&consoleOutb),
		nameSeparator:   "/",
	})
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "file:3:19: upper case character in route\n"

	if consoleOut != expectedConsoleOut {
		t.Errorf("Did not get expected output, got\n%v\n", consoleOut)
	}
}

func TestUpperCaseReportingAllowUpperCase(t *testing.T) {
	const file = `
r /
  route /foo/bar/aMP
	`

	var outb strings.Builder
	var consoleOutb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{"file"},
		output:          "",
		filter:          "",
		verbose:         false,
		allowUpperCase:  true,
		withReader:      mockMultifileReader(map[string]string{"file": file}),
		withWriter:      mockWriter(&outb),
		fprintf:         getAccumFprintf(&consoleOutb),
		nameSeparator:   "/",
	})
	if exitCode != 0 {
		t.Errorf("Expected 0 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != `{"constantPortionNGroups":5,"constantPortionRegexp":"^(?:\\/+(?:(?:(?:(foo)(\\/)\\/*(bar)(\\/)\\/*(aMP)\\/*))))(?:\\?[^#]*)?(?:#.*)?$","families":{"foo/bar/aMP":{"matchRegexp":"^(?:(\\/+foo\\/+bar\\/+aMP\\/*))(\\?[^#]*)?(#.*)?$","nLevels":1,"nonparamGroupNumbers":[1],"members":[{"name":"r/route","paramGroupNumbers":{},"tags":[],"methods":["GET"]}]}}}` {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "\n1 route\n"

	if consoleOut != expectedConsoleOut {
		t.Errorf("Did not get expected output, got\n%v\n", consoleOut)
	}
}

func TestUpperCaseReportingSplitLine(t *testing.T) {
	const file = `
r /
  route /foo/\
    bar/aMP
	`

	var outb strings.Builder
	var consoleOutb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{"file"},
		output:          "",
		filter:          "",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockMultifileReader(map[string]string{"file": file}),
		withWriter:      mockWriter(&outb),
		fprintf:         getAccumFprintf(&consoleOutb),
		nameSeparator:   "/",
	})
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "file:4:10: upper case character in route\n"

	if consoleOut != expectedConsoleOut {
		t.Errorf("Did not get expected output, got\n%v\n", consoleOut)
	}
}

func TestOverlapErrorReportingSimpleCase(t *testing.T) {
	const file1 = "aroute /foo/bar\n"
	const file2 = "broute /afoo/bar\n"
	const file3 = "croute /bfoo/bar\n"
	const file4 = "droute /cfoo/bar\n"
	const file5 = "eroute /foo/bar\n"
	const file6 = "froute /dfoo/bar\n"
	const file7 = "groute /efoo/bar\n"

	var outb strings.Builder
	var consoleOutb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{"file1", "file2", "file3", "file4", "file5", "file6", "file7"},
		output:          "",
		filter:          "",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockMultifileReader(map[string]string{"file1": file1, "file2": file2, "file3": file3, "file4": file4, "file5": file5, "file6": file6, "file7": file7}),
		withWriter:      mockWriter(&outb),
		fprintf:         getAccumFprintf(&consoleOutb),
		nameSeparator:   "/",
	})
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "file1:1: (and file5:1): routes overlap\n"

	if consoleOut != expectedConsoleOut {
		t.Errorf("Did not get expected output, got\n%v\n", consoleOut)
	}
}

func TestOverlapErrorReportingMultiline(t *testing.T) {
	const file1 = "a /line\naroute /foo/bar\nb /glob\n"
	const file2 = "broute /afoo/bar\n"
	const file3 = "croute /bfoo/bar\nxx /another/over\nyy /another/over\n"
	const file4 = "droute /cfoo/bar\n"
	const file5 = "eroute /foo/bar\n"
	const file6 = "froute /dfoo/bar\n"
	const file7 = "groute /efoo/bar\n"

	var outb strings.Builder
	var consoleOutb strings.Builder
	exitCode := run(runParams{
		fancyInputFiles: []string{"file1", "file2", "file3", "file4", "file5", "file6", "file7"},
		output:          "",
		filter:          "",
		verbose:         false,
		allowUpperCase:  false,
		withReader:      mockMultifileReader(map[string]string{"file1": file1, "file2": file2, "file3": file3, "file4": file4, "file5": file5, "file6": file6, "file7": file7}),
		withWriter:      mockWriter(&outb),
		fprintf:         getAccumFprintf(&consoleOutb),
		nameSeparator:   "/",
	})
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "file1:2: (and file5:1): routes overlap\n" +
		"file3:2: (and file3:3): routes overlap\n"

	if consoleOut != expectedConsoleOut {
		t.Errorf("Did not get expected output, got\n%v\n", consoleOut)
	}
}

func valuesOf[T any](nodes []*jsonquery.Node) []T {
	values := make([]T, len(nodes))
	for i := range nodes {
		values[i] = nodes[i].Value().(T)
	}
	return values
}

func mockReader(constant string) func(string, func(io.Reader)) error {
	return func(_ string, f func(io.Reader)) error {
		f(strings.NewReader(constant))
		return nil
	}
}

func mockMultifileReader(contents map[string]string) func(string, func(io.Reader)) error {
	return func(filename string, f func(io.Reader)) error {
		fcont, ok := contents[filename]
		if !ok {
			return fmt.Errorf("Expected to find contents for %v in mockMultifileReader", filename)
		}
		f(strings.NewReader(fcont))
		return nil
	}
}

func mockWriter(out *strings.Builder) func(string, func(io.Writer)) error {
	return func(_ string, f func(io.Writer)) error {
		f(out)
		return nil
	}
}

func dummyFprintf(io.Writer, string, ...interface{}) (int, error) {
	return 0, nil
}

func getAccumFprintf(sb *strings.Builder) func(io.Writer, string, ...interface{}) (int, error) {
	return func(_ io.Writer, fmtString string, args ...interface{}) (int, error) {
		sb.WriteString(fmt.Sprintf(fmtString, args...))
		return 0, nil
	}
}
