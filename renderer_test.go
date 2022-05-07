package markdown

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"

	"github.com/rhysd/go-fakeio"
	"github.com/stretchr/testify/assert"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	transformer = testHelperASTTransformer{}
	md          = goldmark.New(
		goldmark.WithRenderer(NewRenderer()),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(&transformer, 0))),
	)
)

// testHelperASTTransformer is a goldmark AST transformer that helps with debugging failed tests.
type testHelperASTTransformer struct {
	lastDocument *ast.Document
}

// Transform implements goldmark.parser.ASTTransformer
func (t *testHelperASTTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	t.lastDocument = node
}

// DumpLastAST calls Node.Dump() on the most recently parsed AST and returns its output.
func (t *testHelperASTTransformer) DumpLastAST(source []byte) string {
	// Node.Dump() is hardcoded to write to stdout. Fake stdout to get the output as a string.
	result, err := fakeio.Stdout().Do(func() {
		transformer.lastDocument.Dump(source, 0)
	})
	if err != nil {
		panic(err)
	}
	return result
}

// TestRenderedOutput tests that the renderer produces the expected output for all test cases
func TestRenderedOutput(t *testing.T) {
	var testCases = []struct {
		name     string
		options  []Option
		source   string
		expected string
	}{
		// Document
		{
			"Adds trailing newline",
			[]Option{},
			"",
			"\n",
		},
		{
			"Existing trailing newline",
			[]Option{},
			"\n",
			"\n",
		},
		// Headings
		{
			"Setext to ATX heading",
			[]Option{WithHeadingStyle(HeadingStyleATX)},
			"Foo\n---",
			"## Foo\n",
		},
		{
			"ATX to setext heading",
			[]Option{WithHeadingStyle(HeadingStyleSetext)},
			"## FooBar",
			"FooBar\n---\n",
		},
		{
			"Full width setext heading",
			[]Option{WithHeadingStyle(HeadingStyleFullWidthSetext)},
			"Foo Bar\n---",
			"Foo Bar\n-------\n",
		},
		{
			"ATX heading with closing sequence",
			[]Option{WithHeadingStyle(HeadingStyleATXSurround)},
			"## Foo",
			"## Foo ##\n",
		},
		{
			"Empty ATX heading with closing sequence",
			[]Option{WithHeadingStyle(HeadingStyleATXSurround)},
			"##",
			"## ##\n",
		},
		{
			// Setext headings cannot be empty, will always be ATX
			"Empty setext heading",
			[]Option{WithHeadingStyle(HeadingStyleSetext)},
			"##",
			"##\n",
		},
		{
			// ATX headings cannot be multiline, must be setext
			"Multiline ATX heading",
			[]Option{WithHeadingStyle(HeadingStyleATX)},
			"Foo\nBar\n---",
			"Foo\nBar\n---\n",
		},
		// Code Block
		{
			"Space indented code block",
			[]Option{},
			"    foo",
			"    foo\n",
		},
		{
			"Tab indented code block",
			[]Option{WithIndentStyle(IndentStyleTabs)},
			"    foo",
			"\tfoo\n",
		},
		{
			"Multiline code block",
			[]Option{WithIndentStyle(IndentStyleSpaces)},
			"\tfoo\n\tbar\n\tbaz",
			"    foo\n    bar\n    baz\n",
		},
		// Code Span
		{
			"Simple code span",
			[]Option{},
			"`foo`",
			"`foo`\n",
		},
		{
			"Two-backtick code span",
			[]Option{},
			"``foo ` bar``",
			"``foo ` bar``\n",
		},
		{
			"Code span stripping leading and trailing spaces",
			[]Option{},
			"` `` `",
			"````\n",
		},
		{
			"Code span stripping one space",
			[]Option{},
			"`  ``  `",
			"` `` `\n",
		},
		{
			"Unstrippable left space only",
			[]Option{},
			"` a`",
			"` a`\n",
		},
		{
			"Unstrippable only spaces",
			[]Option{},
			"` `\n`  `",
			"` `\n`  `\n",
		},
		{
			"Multiple line-endings treated as spaces",
			[]Option{},
			"``\nfoo\nbar  \nbaz\n``",
			"`foo bar   baz`\n",
		},
		{
			"Line-ending treated as space",
			[]Option{},
			"``\nfoo \n``",
			"`foo `\n",
		},
		{
			"Interior spaces are not collapsed",
			[]Option{},
			"`foo   bar \nbaz`",
			"`foo   bar  baz`\n",
		},
		{
			"Backlashes are treated literally",
			[]Option{},
			"`foo\\`bar`",
			"`foo\\`bar`\n",
		},
		{
			"Two backticks act as delimiters",
			[]Option{},
			"``foo`bar``",
			"``foo`bar``\n",
		},
		{
			"Two backtics inside single ones with spaces trimmed",
			[]Option{},
			"` foo `` bar `",
			"`foo `` bar`\n",
		},
		{
			"Codespan backticks have precedence over emphasis",
			[]Option{},
			"*foo`*`",
			"*foo`*`\n",
		},
		{
			"Codespan backticks have equal precedence with HTML",
			[]Option{},
			"`<a href=\"`\">`",
			"`<a href=\"`\">`\n",
		},
		// TODO: support KindRawHTML
		// {
		// 	"HTML tag with backtick",
		// 	[]Option{},
		// 	"<a href=\"`\">`",
		// 	"<a href=\"`\">`",
		// },
		{
			"Autolink split by a backtick",
			[]Option{},
			"`<http://foo.bar.`baz>`",
			"`<http://foo.bar.`baz>`\n",
		},
		// TODO: support KindAutoLink
		// {
		// 	"Autolink with a backtick",
		// 	[]Option{},
		// 	"<http://foo.bar.`baz>`",
		// 	"<http://foo.bar.`baz>`\n",
		// },
		{
			"Unbalanced 3-2 backticks remain intact",
			[]Option{},
			"```foo``",
			"```foo``\n",
		},
		{
			"Unbalanced 1-0 backticks remain intact",
			[]Option{},
			"`foo",
			"`foo\n",
		},
		{
			"Unbalanced double backticks",
			[]Option{},
			"`foo``bar``",
			"`foo`bar`\n",
		},
		// Paragraph
		{
			"Simple paragraph",
			[]Option{},
			"foo",
			"foo\n",
		},
		{
			"Paragraph with escaped characters",
			[]Option{},
			"\\# foo \\*bar\\* \\__baz\\_\\_",
			"\\# foo \\*bar\\* \\__baz\\_\\_\n",
		},
		// Thematic Break
		{
			"Thematic break default style",
			[]Option{},
			"---",
			"---\n",
		},
		{
			"Thematic break underline style",
			[]Option{WithThematicBreakStyle(ThematicBreakStyleUnderlined)},
			"---",
			"___\n",
		},
		{
			"Thematic break starred style",
			[]Option{WithThematicBreakStyle(ThematicBreakStyleStarred)},
			"---",
			"***\n",
		},
		{
			// Thematic breaks are a minimum of three characters
			"Thematic break zero value",
			[]Option{WithThematicBreakLength(ThematicBreakLength(0))},
			"---",
			"---\n",
		},
		{
			"Thematic break longer length",
			[]Option{WithThematicBreakLength(ThematicBreakLength(10))},
			"---",
			"----------\n",
		},
		// Fenced Code Block
		{
			"Fenced Code Block",
			[]Option{},
			"```\nfoo\nbar\nbaz\n```",
			"```\nfoo\nbar\nbaz\n```\n",
		},
		{
			"Fenced Code Block with language",
			[]Option{},
			"```ruby\ndef foo(x)\n  return 3\nend\n```",
			"```ruby\ndef foo(x)\n  return 3\nend\n```\n",
		},
		{
			"Fenced Code Block with special chars",
			[]Option{},
			"```\n!@#$%^&*\\[],./;'()\n```",
			"```\n!@#$%^&*\\[],./;'()\n```\n",
		},
		// HTML blocks
		{
			"HTML Block Type 1",
			[]Option{},
			"<pre>\nfoo\n</pre>",
			"<pre>\nfoo\n</pre>\n",
		},
		{
			"HTML Block Type 2",
			[]Option{},
			"<!--\ncomment\n-->",
			"<!--\ncomment\n-->\n",
		},
		{
			"HTML Block Type 3",
			[]Option{},
			"<?\nfoo\n?>",
			"<?\nfoo\n?>\n",
		},
		{
			"HTML Block Type 4",
			[]Option{},
			"<!FOO\n!>",
			"<!FOO\n!>\n",
		},
		{
			"HTML Block Type 5",
			[]Option{},
			"<![CDATA[\nfoo\n]]>",
			"<![CDATA[\nfoo\n]]>\n",
		},
		{
			"HTML Block Type 6",
			[]Option{},
			"<hr />",
			"<hr />\n",
		},
		{
			"HTML Block Type 7",
			[]Option{},
			"</a>",
			"</a>\n",
		},
		// Lists
		{
			"Unordered list",
			[]Option{WithIndentStyle(IndentStyleTabs)},
			"- A1\n- B1\n\t- C2\n\t\t- D3\n- E1",
			"- A1\n- B1\n\t- C2\n\t\t- D3\n- E1\n",
		},
		// TODO ordered list
		// Block separators
		{
			"ATX heading block separator",
			[]Option{},
			"# Foo\n# Bar\n\n# Baz",
			"# Foo\n# Bar\n\n# Baz\n",
		},
		{
			"Setext heading block separator",
			[]Option{WithHeadingStyle(HeadingStyleSetext)},
			"Foo\n---\nBar\n---\n\nBaz\n---",
			"Foo\n---\nBar\n---\n\nBaz\n---\n",
		},
		{
			"Code block separator",
			[]Option{WithIndentStyle(IndentStyleTabs)},
			"\tcode 1\n---\n\tcode 2\n---\n\n\tcode 3",
			"\tcode 1\n---\n\tcode 2\n---\n\n\tcode 3\n",
		},
		{
			"Fenced code block separator",
			[]Option{},
			"```\ncode 1\n```\n```\ncode 2\n```\n\n```\ncode 3\n```",
			"```\ncode 1\n```\n```\ncode 2\n```\n\n```\ncode 3\n```\n",
		},
		{
			"HTML block separator",
			[]Option{},
			"<?foo?>\n<?bar?>\n\n<?baz?>",
			"<?foo?>\n<?bar?>\n\n<?baz?>\n",
		},
		{
			"List block separator",
			[]Option{},
			"- foo\n+ bar\n\n* baz",
			"- foo\n+ bar\n\n* baz\n",
		},
		{
			"List item block separator",
			[]Option{},
			"- foo\n- bar\n\n- baz",
			"- foo\n- bar\n\n- baz\n",
		},
		{
			"Text block separator",
			[]Option{},
			"- foo\n- bar\n\n- baz",
			"- foo\n- bar\n\n- baz\n",
		},

		// Tight and "loose" lists
		{
			"Tight list",
			[]Option{},
			"Paragraph\n- A1\n- B1",
			"Paragraph\n- A1\n- B1\n",
		},
		{
			"Loose list",
			[]Option{},
			"Paragraph\n\n- A1\n- B1",
			"Paragraph\n\n- A1\n- B1\n",
		},
		// Links
		{
			"Empty Link",
			[]Option{},
			"[]()",
			"[]()\n",
		},
		{
			"Link",
			[]Option{},
			"[link](/uri)",
			"[link](/uri)\n",
		},
		{
			"Link with title",
			[]Option{},
			"[link](/uri \"title\")",
			"[link](/uri \"title\")\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			buf := bytes.Buffer{}

			renderer := NewRenderer(tc.options...)
			md.SetRenderer(renderer)
			err := md.Convert([]byte(tc.source), &buf)
			assert.NoError(err)
			assert.Equal(tc.expected, buf.String())
			t.Logf("Markdown source: %q\n", tc.source)
			t.Log(transformer.DumpLastAST([]byte(tc.source)))
		})
	}
}

// errorBuf implements util.BufWriter and returns errors for all write operations.
type errorBuf struct {
	util.BufWriter
	lastError  error
	numWritten int
}

func (e *errorBuf) Write(p []byte) (n int, err error) {
	return e.numWritten, e.lastError
}
func (e *errorBuf) Available() int {
	return 0
}
func (e *errorBuf) Buffered() int {
	return 0
}
func (e *errorBuf) Flush() error {
	return e.lastError
}
func (e *errorBuf) WriteByte(c byte) error {
	return e.lastError
}

func (e *errorBuf) WriteRune(r rune) (size int, err error) {
	return e.numWritten, e.lastError
}

func (e *errorBuf) WriteString(s string) (int, error) {
	return e.numWritten, e.lastError
}

// TestRenderWriter tests the renderer's implementation of goldmark's util.BufWriter interface,
// a subset of bufio.Writer.
func TestRenderWriter(t *testing.T) {
	bufBytes := bytes.Buffer{}
	bufWriter := bufio.NewWriter(&bufBytes)
	writer := renderWriter{}
	assert := assert.New(t)

	// WriteString
	data := "foobar"
	writer.WriteString(bufWriter, data)
	assert.NoError(bufWriter.Flush())
	assert.Equal(bufBytes.String(), data)
	assert.Equal(writer.lastWrittenByte, data[len(data)-1])
	assert.NoError(writer.err)

	bufBytes.Reset()

	// Write
	data2 := []byte("raboof")
	writer.Write(bufWriter, data2)

	assert.NoError(bufWriter.Flush())
	assert.Equal(bufBytes.Bytes(), data2)
	assert.Equal(writer.lastWrittenByte, data2[len(data2)-1])
	assert.NoError(writer.err)

	// Write with error
	errString := "test error"
	errWriter := errorBuf{lastError: fmt.Errorf(errString), numWritten: 1}
	data3 := "zxyq"
	writer.WriteString(&errWriter, data3)

	assert.EqualError(writer.err, errString)
	assert.Equal(string(writer.lastWrittenByte), string(data3[errWriter.numWritten-1]))

	bufBytes.Reset()

	// Further writes are no-ops
	writer.WriteString(bufWriter, data)
	writer.Write(bufWriter, data2)

	assert.NoError(bufWriter.Flush())
	assert.EqualError(writer.err, errString)
	assert.Equal(bufBytes.Bytes(), []byte{})
	assert.Equal(string(writer.lastWrittenByte), string(data3[errWriter.numWritten-1]))

	// Reset clears error state
	writer.Reset()
	assert.NoError(writer.err)
	assert.Equal(writer.lastWrittenByte, byte(0))
}
