/*

Provides the mechanism for managing a given Docker container-based lambda.

Must be paired with a DockerSandboxManager which handles pulling handler
code, initializing containers, etc.

*/

package sandbox

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-lambda/open-lambda/worker/config"
	"github.com/open-lambda/open-lambda/worker/handler/state"
)

type DockerSandbox struct {
	name        string
	sandbox_dir string
	nspid       int
	container   *docker.Container
	client      *docker.Client
	config      *config.Config
	controllers string
}

func NewDockerSandbox(name string, sandbox_dir string, container *docker.Container, client *docker.Client, config *config.Config) *DockerSandbox {
	sandbox := &DockerSandbox{
		name:        name,
		sandbox_dir: sandbox_dir,
		container:   container,
		client:      client,
		config:      config,
        // name=systemd?
		controllers: "memory,cpu,devices,perf_event,cpuset,blkio,pids,freezer,net_cls,net_prio,hugetlb",
	}

	return sandbox
}

func (s *DockerSandbox) dockerError(outer error) (err error) {
	buf := bytes.NewBufferString(outer.Error() + ".  ")

	if err := s.InspectUpdate(); err != nil {
		buf.WriteString(fmt.Sprintf("Could not inspect container (%v).  ", err.Error()))
	} else {
		buf.WriteString(fmt.Sprintf("Container state is <%v>.  ", s.container.State.StateString()))
	}

	if log, err := s.Logs(); err != nil {
		buf.WriteString(fmt.Sprintf("Could not fetch [%s] logs!\n", s.container.ID))
	} else {
		buf.WriteString(fmt.Sprintf("<--- Start handler container [%s] logs: --->\n", s.container.ID))
		buf.WriteString(log)
		buf.WriteString(fmt.Sprintf("<--- End handler container [%s] logs --->\n", s.container.ID))
	}

	return errors.New(buf.String())
}

func (s *DockerSandbox) InspectUpdate() error {
	container, err := s.client.InspectContainer(s.container.ID)
	if err != nil {
		return err
	}
	s.container = container

	return nil
}

func (s *DockerSandbox) State() (hstate state.HandlerState, err error) {
	if err := s.InspectUpdate(); err != nil {
		return hstate, err
	}

	if s.container.State.Running {
		if s.container.State.Paused {
			hstate = state.Paused
		} else {
			hstate = state.Running
		}
	} else {
		hstate = state.Stopped
	}

	return hstate, nil
}

func (s *DockerSandbox) Channel() (channel *SandboxChannel, err error) {
	if err := s.InspectUpdate(); err != nil {
		return nil, s.dockerError(err)
	}

	dial := func(proto, addr string) (net.Conn, error) {
		return net.Dial("unix", filepath.Join(s.sandbox_dir, "ol.sock"))
	}
	tr := http.Transport{Dial: dial}

	// the server name doesn't matter since we have a sock file
	return &SandboxChannel{Url: "http://container", Transport: tr}, nil
}

/* Starts the container */
func (s *DockerSandbox) Start() error {
	if err := s.client.StartContainer(s.container.ID, nil); err != nil {
		log.Printf("failed to start container with err %v\n", err)
		return s.dockerError(err)
	}

	container, err := s.client.InspectContainer(s.container.ID)
	if err != nil {
		log.Printf("failed to inpect container with err %v\n", err)
		return s.dockerError(err)
	}
	s.container = container
	s.nspid = container.State.Pid

	return nil
}

/* Stops the container */
func (s *DockerSandbox) Stop() error {
	// TODO(tyler): is there any advantage to trying to stop
	// before killing?  (i.e., use SIGTERM instead SIGKILL)
	opts := docker.KillContainerOptions{ID: s.container.ID}
	if err := s.client.KillContainer(opts); err != nil {
		log.Printf("failed to kill container with error %v\n", err)
		return s.dockerError(err)
	}

	return nil
}

/* Pauses the container */
func (s *DockerSandbox) Pause() error {

	if err := s.client.PauseContainer(s.container.ID); err != nil {
		log.Printf("failed to pause container with error %v\n", err)
		return s.dockerError(err)
	}

	return nil
}

/* Unpauses the container */
func (s *DockerSandbox) Unpause() error {
	if err := s.client.UnpauseContainer(s.container.ID); err != nil {
		log.Printf("failed to unpause container %s with err %v\n", s.name, err)
		return s.dockerError(err)
	}

	return nil
}

/* Frees all resources associated with the lambda (stops the container if necessary) */
func (s *DockerSandbox) Remove() error {
	if err := s.client.RemoveContainer(docker.RemoveContainerOptions{
		ID: s.container.ID,
	}); err != nil {
		log.Printf("failed to rm container with err %v", err)
		return s.dockerError(err)
	}

	return nil
}

/* Return log output for the container */
func (s *DockerSandbox) Logs() (string, error) {
	stdout_path := filepath.Join(s.sandbox_dir, "stdout")
	stderr_path := filepath.Join(s.sandbox_dir, "stderr")

	stdout, err := ioutil.ReadFile(stdout_path)
	if err != nil {
		return "", err
	}

	stderr, err := ioutil.ReadFile(stderr_path)
	if err != nil {
		return "", err
	}

	stdout_hdr := fmt.Sprintf("Container (%s) stdout:", s.container.ID)
	stderr_hdr := fmt.Sprintf("Container (%s) stderr:", s.container.ID)
	ret := fmt.Sprintf("%s\n%s\n%s\n%s\n", stdout_hdr, stdout, stderr_hdr, stderr)

	return ret, nil
}

func (s *DockerSandbox) CGroupEnter(pid string) (err error) {
    cgroup := fmt.Sprintf("%s:/docker/%s", s.controllers, s.container.ID)
    cmd := exec.Command("cgclassify", "--sticky", "-g", cgroup, pid)

    if err := cmd.Run(); err != nil {
        return err
    }

	return nil
}

func (s *DockerSandbox) NSPid() int {
	return s.nspid
}
