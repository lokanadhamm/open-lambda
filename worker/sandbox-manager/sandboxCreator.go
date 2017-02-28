package sbmanager

/*

Contains sandboxCreator and its implementations. They are used by SandboxManager
to delegate the Create() call. All are private since user should not care about
the implementation details, but only the expected bebaviors of the manager.

*/

import (
	"fmt"
	"log"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-lambda/open-lambda/worker/dockerutil"
	sb "github.com/open-lambda/open-lambda/worker/sandbox"
)

// sandboxCreator creates sandboxes.
type sandboxCreator interface {
	Create(name string, sandbox_dir string) (sb.Sandbox, error)
}

// dockerSbCreator wraps necessary information for creating sandboxes in Docker
// containers.
type dockerSbCreator struct {
	labels map[string]string // Identifies cluster and type
	env    []string          // TODO: could be removed as we have socket file
	client *docker.Client
}

// init initializes a dockerSbCreator.
func (dc *dockerSbCreator) init(client *docker.Client, labels map[string]string, env []string) {
	dc.client = client
	dc.labels = labels
	dc.env = env
}

// namedImgSbCreator creates sandboxes with Docker images that has the same
// name as the handler.
type namedImgSbCreator struct {
	dockerSbCreator
}

// baseImgSbCreator creates sandboxes with the base image and mounts the
// handler and necessary files into the container.
type baseImgSbCreator struct {
	dockerSbCreator
	handler_dir string
}

// newNamedImgSbCreator creates a namedImgSbCreator.
func newNamedImgSbCreator(client *docker.Client, labels map[string]string, env []string) *namedImgSbCreator {
	nc := new(namedImgSbCreator)
	nc.init(client, labels, env)
	return nc
}

// Create creates a sandbox with a container using a named image.
func (nc *namedImgSbCreator) Create(name string, sandbox_dir string) (sb.Sandbox, error) {
	volumes := []string{fmt.Sprintf("%s:%s", sandbox_dir, "/host")}
	if container, err := dockerutil.Create(nc.client, name, nc.labels, nc.env, volumes); err != nil {
		return nil, err
	} else {
		// TODO: ok to pass nil for opts? should be removed later
		sandbox := sb.NewDockerSandbox(name, sandbox_dir, container, nc.client, nil)
		return sandbox, nil
	}
}

// newBaseImgSbCreator creates a baseImgSbCreator.
func newBaseImgSbCreator(client *docker.Client, labels map[string]string, env []string, handler_dir string) *baseImgSbCreator {
	if err := dockerutil.AssertImageExists(client, BASE_IMAGE); err != nil {
		log.Fatalf("Docker image %s does not exist", BASE_IMAGE)
	}
	bc := new(baseImgSbCreator)
	bc.init(client, labels, env)
	bc.handler_dir = handler_dir
	return bc
}

// Create creates a sandbox using the base image mounted with the local handler
// directory.
func (bc *baseImgSbCreator) Create(name string, sandbox_dir string) (sb.Sandbox, error) {
	handler := filepath.Join(bc.handler_dir, name)
	volumes := []string{
		fmt.Sprintf("%s:%s", handler, "/handler"),
		fmt.Sprintf("%s:%s", sandbox_dir, "/host")}
	if container, err := dockerutil.Create(bc.client, BASE_IMAGE, bc.labels, bc.env, volumes); err != nil {
		return nil, err
	} else {
		sandbox := sb.NewDockerSandbox(name, sandbox_dir, container, bc.client, nil)
		return sandbox, nil
	}
}
