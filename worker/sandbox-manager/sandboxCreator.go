package sbmanager

import (
	"fmt"
	"log"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-lambda/open-lambda/worker/dockerutil"
	sb "github.com/open-lambda/open-lambda/worker/sandbox"
)

type sandboxCreator interface {
	Create(name string, sandbox_dir string) (sb.Sandbox, error)
}

type dockerSbCreator struct {
	labels map[string]string
	env    []string
	client *docker.Client
}

func (dc *dockerSbCreator) init(client *docker.Client, labels map[string]string, env []string) {
	dc.client = client
	dc.labels = labels
	dc.env = env
}

type namedImgSbCreator struct {
	dockerSbCreator
}

type baseImgSbCreator struct {
	dockerSbCreator
	handler_dir string
}

func newNamedImgSbCreator(client *docker.Client, labels map[string]string, env []string) *namedImgSbCreator {
	nc := new(namedImgSbCreator)
	nc.init(client, labels, env)
	return nc
}

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

func newBaseImgSbCreator(client *docker.Client, labels map[string]string, env []string, handler_dir string) *baseImgSbCreator {
	if err := dockerutil.AssertImageExists(client, BASE_IMAGE); err != nil {
		log.Fatalf("Docker image %s does not exist", BASE_IMAGE)
	}
	bc := new(baseImgSbCreator)
	bc.init(client, labels, env)
	bc.handler_dir = handler_dir
	return bc
}

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
