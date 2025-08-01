package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/moby/moby/v2/integration-cli/cli"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerCLIInfoSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIInfoSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLIInfoSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}

// ensure docker info succeeds
func (s *DockerCLIInfoSuite) TestInfoEnsureSucceeds(c *testing.T) {
	out := cli.DockerCmd(c, "info").Stdout()

	// always shown fields
	stringsToCheck := []string{
		"ID:",
		"Containers:",
		" Running:",
		" Paused:",
		" Stopped:",
		"Images:",
		"OSType:",
		"Architecture:",
		"Logging Driver:",
		"Operating System:",
		"CPUs:",
		"Total Memory:",
		"Kernel Version:",
		"Storage Driver:",
		"Volume:",
		"Network:",
		"Live Restore Enabled:",
	}

	if testEnv.DaemonInfo.OSType == "linux" {
		stringsToCheck = append(stringsToCheck, "Init Binary:", "Security Options:", "containerd version:", "runc version:", "init version:")
	}

	if DaemonIsLinux() {
		stringsToCheck = append(stringsToCheck, "Runtimes:", "Default Runtime: runc")
	}

	if testEnv.DaemonInfo.ExperimentalBuild {
		stringsToCheck = append(stringsToCheck, "Experimental: true")
	} else {
		stringsToCheck = append(stringsToCheck, "Experimental: false")
	}

	for _, linePrefix := range stringsToCheck {
		assert.Assert(c, strings.Contains(out, linePrefix), "couldn't find string %v in output", linePrefix)
	}
}

func (s *DockerCLIInfoSuite) TestInfoDisplaysRunningContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	existing := existingContainerStates(c)

	cli.DockerCmd(c, "run", "-d", "busybox", "top")
	out := cli.DockerCmd(c, "info").Stdout()
	assert.Assert(c, is.Contains(out, fmt.Sprintf("Containers: %d\n", existing["Containers"]+1)))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Running: %d\n", existing["ContainersRunning"]+1)))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Paused: %d\n", existing["ContainersPaused"])))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Stopped: %d\n", existing["ContainersStopped"])))
}

func (s *DockerCLIInfoSuite) TestInfoDisplaysPausedContainers(c *testing.T) {
	testRequires(c, IsPausable)

	existing := existingContainerStates(c)

	id := runSleepingContainer(c, "-d")

	cli.DockerCmd(c, "pause", id)

	out := cli.DockerCmd(c, "info").Stdout()
	assert.Assert(c, is.Contains(out, fmt.Sprintf("Containers: %d\n", existing["Containers"]+1)))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Running: %d\n", existing["ContainersRunning"])))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Paused: %d\n", existing["ContainersPaused"]+1)))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Stopped: %d\n", existing["ContainersStopped"])))
}

func (s *DockerCLIInfoSuite) TestInfoDisplaysStoppedContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	existing := existingContainerStates(c)

	out := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	cleanedContainerID := strings.TrimSpace(out)

	cli.DockerCmd(c, "stop", cleanedContainerID)

	out = cli.DockerCmd(c, "info").Stdout()
	assert.Assert(c, is.Contains(out, fmt.Sprintf("Containers: %d\n", existing["Containers"]+1)))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Running: %d\n", existing["ContainersRunning"])))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Paused: %d\n", existing["ContainersPaused"])))
	assert.Assert(c, is.Contains(out, fmt.Sprintf(" Stopped: %d\n", existing["ContainersStopped"]+1)))
}

func existingContainerStates(t *testing.T) map[string]int {
	out := cli.DockerCmd(t, "info", "--format", "{{json .}}").Stdout()
	var m map[string]interface{}
	err := json.Unmarshal([]byte(out), &m)
	assert.NilError(t, err)
	res := map[string]int{}
	res["Containers"] = int(m["Containers"].(float64))
	res["ContainersRunning"] = int(m["ContainersRunning"].(float64))
	res["ContainersPaused"] = int(m["ContainersPaused"].(float64))
	res["ContainersStopped"] = int(m["ContainersStopped"].(float64))
	return res
}
