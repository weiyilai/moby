//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/moby/moby/client/pkg/stringid"
	"github.com/moby/moby/v2/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// user namespaces test: run daemon with remapped root setting
// 1. validate uid/gid maps are set properly
// 2. verify that files created are owned by remapped root
func (s *DockerDaemonSuite) TestDaemonUserNamespaceRootSetting(c *testing.T) {
	testRequires(c, UserNamespaceInKernel)

	ctx := testutil.GetContext(c)
	s.d.StartWithBusybox(ctx, c, "--userns-remap", "default")

	out, err := s.d.Cmd("run", "busybox", "stat", "-c", "%u:%g", "/bin/cat")
	assert.Check(c, err)
	assert.Assert(c, is.Equal(strings.TrimSpace(out), "0:0"))

	tmpDir, err := os.MkdirTemp("", "userns")
	assert.NilError(c, err)

	defer os.RemoveAll(tmpDir)

	// Set a non-existent path
	tmpDirNotExists := path.Join(os.TempDir(), "userns"+stringid.GenerateRandomID())
	defer os.RemoveAll(tmpDirNotExists)

	// we need to find the uid and gid of the remapped root from the daemon's root dir info
	uidgid := strings.Split(filepath.Base(s.d.Root), ".")
	assert.Equal(c, len(uidgid), 2, fmt.Sprintf("Should have gotten uid/gid strings from root dirname: %s", filepath.Base(s.d.Root)))
	uid, err := strconv.Atoi(uidgid[0])
	assert.NilError(c, err, "Can't parse uid")
	gid, err := strconv.Atoi(uidgid[1])
	assert.NilError(c, err, "Can't parse gid")

	// writable by the remapped root UID/GID pair
	assert.NilError(c, os.Chown(tmpDir, uid, gid))

	out, err = s.d.Cmd("run", "-d", "--name", "userns", "-v", tmpDir+":/goofy", "-v", tmpDirNotExists+":/donald", "busybox", "sh", "-c", "touch /goofy/testfile; exec top")
	assert.NilError(c, err, "Output: %s", out)

	user := s.findUser(c, "userns")
	assert.Equal(c, uidgid[0], user)

	// check that the created directory is owned by remapped uid:gid
	statNotExists, err := os.Stat(tmpDirNotExists)
	assert.NilError(c, err)
	fi := statNotExists.Sys().(*syscall.Stat_t)
	assert.Equal(c, fi.Uid, uint32(uid), "Created directory not owned by remapped root UID")
	assert.Equal(c, fi.Gid, uint32(gid), "Created directory not owned by remapped root GID")

	pid, err := s.d.Cmd("inspect", "--format={{.State.Pid}}", "userns")
	assert.Assert(c, err == nil, "Could not inspect running container: out: %q", pid)
	// check the uid and gid maps for the PID to ensure root is remapped
	// (cmd = cat /proc/<pid>/uid_map | grep -E '0\s+9999\s+1')
	_, err = RunCommandPipelineWithOutput(
		exec.Command("cat", "/proc/"+strings.TrimSpace(pid)+"/uid_map"),
		exec.Command("grep", "-E", fmt.Sprintf("0[[:space:]]+%d[[:space:]]+", uid)))
	assert.NilError(c, err)

	_, err = RunCommandPipelineWithOutput(
		exec.Command("cat", "/proc/"+strings.TrimSpace(pid)+"/gid_map"),
		exec.Command("grep", "-E", fmt.Sprintf("0[[:space:]]+%d[[:space:]]+", gid)))
	assert.NilError(c, err)

	// check that the touched file is owned by remapped uid:gid
	stat, err := os.Stat(filepath.Join(tmpDir, "testfile"))
	assert.NilError(c, err)
	fi = stat.Sys().(*syscall.Stat_t)
	assert.Equal(c, fi.Uid, uint32(uid), "Touched file not owned by remapped root UID")
	assert.Equal(c, fi.Gid, uint32(gid), "Touched file not owned by remapped root GID")

	// use host usernamespace
	out, err = s.d.Cmd("run", "-d", "--name", "userns_skip", "--userns", "host", "busybox", "sh", "-c", "touch /goofy/testfile; exec top")
	assert.Assert(c, err == nil, "Output: %s", out)
	user = s.findUser(c, "userns_skip")
	// userns are skipped, user is root
	assert.Equal(c, user, "root")
}

// findUser finds the uid or name of the user of the first process that runs in a container
func (s *DockerDaemonSuite) findUser(t *testing.T, container string) string {
	out, err := s.d.Cmd("top", container)
	assert.Assert(t, err == nil, "Output: %s", out)
	rows := strings.Split(out, "\n")
	if len(rows) < 2 {
		// No process rows founds
		t.FailNow()
	}
	return strings.Fields(rows[1])[0]
}
