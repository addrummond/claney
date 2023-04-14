package main

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/addrummond/claney/compiler"
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
	exitCode := run([]string{""}, "", "", []compiler.IncludeSpec{}, false, mockReader(exampleInput), mockWriter(&outb), dummyFprintf, "/")
	if exitCode != 0 {
		t.Errorf("Expected 0 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	doc, err := jsonquery.Parse(strings.NewReader(out))
	if err != nil {
		t.Errorf("%v", err)
	}
	names := valuesOf[string](jsonquery.Find(doc, "/families/*/members/*/name"))
	if !reflect.DeepEqual(names, []string{"root/api", "root/api/getstuff", "root/manager/settings"}) {
		t.Errorf("Expected all routes to be included in output, got %+v\n", names)
	}
}

func TestOutputPrefix(t *testing.T) {
	var outb strings.Builder
	exitCode := run([]string{""}, "", "export FOO = ", []compiler.IncludeSpec{}, false, mockReader(exampleInput), mockWriter(&outb), dummyFprintf, "/")
	if exitCode != 0 {
		t.Errorf("Expected 0 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if !strings.HasPrefix(out, "export FOO = ") {
		t.Errorf("Output string lacked expected prefix")
	}
}

func TestRunExcludeAllTags(t *testing.T) {
	var outb strings.Builder
	exitCode := run([]string{""}, "", "", []compiler.IncludeSpec{{Include: false, TagGlob: "man*"}, {Include: false, TagGlob: "ap*"}}, false, mockReader(exampleInput), mockWriter(&outb), dummyFprintf, "/")
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
	exitCode := run([]string{""}, "", "", []compiler.IncludeSpec{{Include: true, TagGlob: "ap*"}}, false, mockReader(exampleInput), mockWriter(&outb), dummyFprintf, "/")
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
	exitCode := run([]string{"file1", "file2"}, "", "", []compiler.IncludeSpec{}, false, mockMultifileReader(map[string]string{"file1": file1, "file2": file2}), mockWriter(&outb), getAccumFprintf(&consoleOutb), "/")
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "file1 line 3: missing route name or missing route pattern\n" +
		"file2 line 4: missing route name or missing route pattern\n"

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
	exitCode := run([]string{"file1", "file2", "file3", "file4", "file5", "file6", "file7"}, "", "", []compiler.IncludeSpec{}, false, mockMultifileReader(map[string]string{"file1": file1, "file2": file2, "file3": file3, "file4": file4, "file5": file5, "file6": file6, "file7": file7}), mockWriter(&outb), getAccumFprintf(&consoleOutb), "/")
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "(file1 line 1; file5 line 1): routes overlap\n"

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
	exitCode := run([]string{"file1", "file2", "file3", "file4", "file5", "file6", "file7"}, "", "", []compiler.IncludeSpec{}, false, mockMultifileReader(map[string]string{"file1": file1, "file2": file2, "file3": file3, "file4": file4, "file5": file5, "file6": file6, "file7": file7}), mockWriter(&outb), getAccumFprintf(&consoleOutb), "/")
	if exitCode != 1 {
		t.Errorf("Expected 1 exit code, got %v\n", exitCode)
	}
	out := outb.String()
	if out != "" {
		t.Errorf("Unexpected output written:\n%v\n", out)
	}

	consoleOut := consoleOutb.String()
	const expectedConsoleOut = "(file1 line 2; file5 line 1): routes overlap\n" +
		"(file3 line 2; file3 line 3): routes overlap\n"

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
