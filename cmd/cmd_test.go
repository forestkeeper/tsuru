package cmd

import (
	"bytes"
	"errors"
	"github.com/timeredbull/tsuru/fs"
	"github.com/timeredbull/tsuru/fs/testing"
	"io"
	. "launchpad.net/gocheck"
	"os"
	"syscall"
)

type recordingExiter int

func (e *recordingExiter) Exit(code int) {
	*e = recordingExiter(code)
}

func (e recordingExiter) value() int {
	return int(e)
}

func (s *S) patchStdin(c *C, content []byte) {
	f, err := os.OpenFile("/tmp/passwdfile.txt", syscall.O_RDWR|syscall.O_NDELAY|syscall.O_CREAT|syscall.O_TRUNC, 0600)
	c.Assert(err, IsNil)
	n, err := f.Write(content)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, len(content))
	ret, err := f.Seek(0, 0)
	c.Assert(err, IsNil)
	c.Assert(ret, Equals, int64(0))
	s.stdin = os.Stdin
	os.Stdin = f
}

func (s *S) unpatchStdin() {
	os.Stdin = s.stdin
}

type TestCommand struct{}

func (c *TestCommand) Info() *Info {
	return &Info{
		Name:  "foo",
		Desc:  "Foo do anything or nothing.",
		Usage: "foo",
	}
}

func (c *TestCommand) Run(context *Context, client Doer) error {
	io.WriteString(context.Stdout, "Running TestCommand")
	return nil
}

type ErrorCommand struct {
	msg string
}

func (c *ErrorCommand) Info() *Info {
	return &Info{Name: "error"}
}

func (c *ErrorCommand) Run(context *Context, client Doer) error {
	return errors.New(c.msg)
}

func (s *S) TestRegister(c *C) {
	manager.Register(&TestCommand{})
	badCall := func() { manager.Register(&TestCommand{}) }
	c.Assert(badCall, PanicMatches, "command already registered: foo")
}

func (s *S) TestManagerRunShouldWriteErrorsOnStderr(c *C) {
	manager.Register(&ErrorCommand{msg: "You are wrong\n"})
	manager.Run([]string{"error"})
	c.Assert(manager.Stderr.(*bytes.Buffer).String(), Equals, "You are wrong\n")
}

func (s *S) TestManagerRunShouldReturnStatus1WhenCommandFail(c *C) {
	manager.Register(&ErrorCommand{msg: "You are wrong\n"})
	manager.Run([]string{"error"})
	c.Assert(manager.e.(*recordingExiter).value(), Equals, 1)
}

func (s *S) TestManagerRunShouldAppendNewLineOnErrorWhenItsNotPresent(c *C) {
	manager.Register(&ErrorCommand{msg: "You are wrong"})
	manager.Run([]string{"error"})
	c.Assert(manager.Stderr.(*bytes.Buffer).String(), Equals, "You are wrong\n")
}

func (s *S) TestRun(c *C) {
	manager.Register(&TestCommand{})
	manager.Run([]string{"foo"})
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, "Running TestCommand")
}

func (s *S) TestRunCommandThatDoesNotExist(c *C) {
	manager.Run([]string{"bar"})
	c.Assert(manager.Stderr.(*bytes.Buffer).String(), Equals, "command bar does not exist\n")
	c.Assert(manager.e.(*recordingExiter).value(), Equals, 1)
}
func (s *S) TestHelp(c *C) {
	expected := `Usage: glb command [args]

Available commands:
  help
  user-create

Run glb help <commandname> to get more information about a specific command.
`
	manager.Register(&userCreate{})
	context := Context{[]string{}, []string{}, manager.Stdout, manager.Stderr}
	command := help{manager: &manager}
	err := command.Run(&context, nil)
	c.Assert(err, IsNil)
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, expected)
}

func (s *S) TestHelpCommandShouldBeRegisteredByDefault(c *C) {
	var stdout, stderr bytes.Buffer
	m := NewManager("tsuru", &stdout, &stderr)
	_, exists := m.Commands["help"]
	c.Assert(exists, Equals, true)
}

func (s *S) TestRunWithoutArgsShouldRunsHelp(c *C) {
	expected := `Usage: glb command [args]

Available commands:
  help

Run glb help <commandname> to get more information about a specific command.
`
	manager.Run([]string{})
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, expected)
}

func (s *S) TestHelpShouldReturnsHelpForACmd(c *C) {
	expected := `Usage: glb foo

Foo do anything or nothing.
`
	manager.Register(&TestCommand{})
	manager.Run([]string{"help", "foo"})
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, expected)
}

type ArgCmd struct{}

func (c *ArgCmd) Info() *Info {
	return &Info{
		Name:    "arg",
		MinArgs: 1,
		Usage:   "arg [args]",
		Desc:    "some desc",
	}
}

func (cmd *ArgCmd) Run(ctx *Context, client Doer) error {
	return nil
}

func (s *S) TestRunWrongArgsNumberShouldRunsHelpAndReturnStatus1(c *C) {
	expected := `Not enough arguments to call arg.

Usage: glb arg [args]

some desc

Minimum arguments: 1
`
	manager.Register(&ArgCmd{})
	manager.Run([]string{"arg"})
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, expected)
	c.Assert(manager.e.(*recordingExiter).value(), Equals, 1)
}

func (s *S) TestHelpShouldReturnUsageWithTheCommandName(c *C) {
	expected := `Usage: tsuru foo

Foo do anything or nothing.
`
	var stdout, stderr bytes.Buffer
	manager := NewManager("tsuru", &stdout, &stderr)
	manager.Register(&TestCommand{})
	context := Context{[]string{}, []string{"foo"}, manager.Stdout, manager.Stderr}
	command := help{manager: &manager}
	err := command.Run(&context, nil)
	c.Assert(err, IsNil)
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, expected)
}

func (s *S) TestExtractProgramNameWithAbsolutePath(c *C) {
	got := ExtractProgramName("/usr/bin/tsuru")
	c.Assert(got, Equals, "tsuru")
}

func (s *S) TestExtractProgramNameWithRelativePath(c *C) {
	got := ExtractProgramName("./tsuru")
	c.Assert(got, Equals, "tsuru")
}

func (s *S) TestExtractProgramNameWithinThePATH(c *C) {
	got := ExtractProgramName("tsuru")
	c.Assert(got, Equals, "tsuru")
}

func (s *S) TestFinisherReturnsOsExiterIfNotDefined(c *C) {
	m := Manager{}
	c.Assert(m.finisher(), FitsTypeOf, osExiter{})
}

func (s *S) TestFinisherReturnTheDefinedE(c *C) {
	var exiter recordingExiter
	m := Manager{e: &exiter}
	c.Assert(m.finisher(), FitsTypeOf, &exiter)
}

func (s *S) TestLoginIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	lgn, ok := manager.Commands["login"]
	c.Assert(ok, Equals, true)
	c.Assert(lgn, FitsTypeOf, &login{})
}

func (s *S) TestLogoutIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	lgt, ok := manager.Commands["logout"]
	c.Assert(ok, Equals, true)
	c.Assert(lgt, FitsTypeOf, &logout{})
}

func (s *S) TestUserCreateIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	user, ok := manager.Commands["user-create"]
	c.Assert(ok, Equals, true)
	c.Assert(user, FitsTypeOf, &userCreate{})
}

func (s *S) TestTeamCreateIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	create, ok := manager.Commands["team-create"]
	c.Assert(ok, Equals, true)
	c.Assert(create, FitsTypeOf, &teamCreate{})
}

func (s *S) TestTeamListIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	list, ok := manager.Commands["team-list"]
	c.Assert(ok, Equals, true)
	c.Assert(list, FitsTypeOf, &teamList{})
}

func (s *S) TestTeamAddUserIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	adduser, ok := manager.Commands["team-user-add"]
	c.Assert(ok, Equals, true)
	c.Assert(adduser, FitsTypeOf, &teamUserAdd{})
}

func (s *S) TestTeamRemoveUserIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	removeuser, ok := manager.Commands["team-user-remove"]
	c.Assert(ok, Equals, true)
	c.Assert(removeuser, FitsTypeOf, &teamUserRemove{})
}

func (s *S) TestTargetIsRegistered(c *C) {
	manager := BuildBaseManager("tsuru")
	tgt, ok := manager.Commands["target"]
	c.Assert(ok, Equals, true)
	c.Assert(tgt, FitsTypeOf, &target{})
}

func (s *S) TestFileSystem(c *C) {
	fsystem = &testing.RecordingFs{}
	c.Assert(filesystem(), DeepEquals, fsystem)
	fsystem = nil
	c.Assert(filesystem(), DeepEquals, fs.OsFs{})
}
