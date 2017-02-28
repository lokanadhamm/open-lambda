package sbmanager

/*

Defines common variables and functions to be shared
by managers which managing Docker containers.

*/

import (
	"fmt"
	"log"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-lambda/open-lambda/worker/config"
	sb "github.com/open-lambda/open-lambda/worker/sandbox"
)

const (
	DOCKER_LABEL_CLUSTER = "ol.cluster"
	DOCKER_LABEL_TYPE    = "ol.type"
	SANDBOX              = "sandbox"
	BASE_IMAGE           = "lambda"
)

type DockerManager struct {
	client  *docker.Client
	codeMgr codeManager
	creator sandboxCreator
}

func NewDockerManager(opts *config.Config) *DockerManager {
	dm := &DockerManager{}

	// NOTE: This requires a running docker daemon on the host
	c, err := docker.NewClientFromEnv()
	if err != nil {
		log.Fatal("failed to get docker client: ", err)
	} else {
		dm.client = c
	}

	env := []string{fmt.Sprintf("ol.config=%s", opts.SandboxConfJson())}

	labels := map[string]string{}
	labels[DOCKER_LABEL_CLUSTER] = opts.Cluster_name
	labels[DOCKER_LABEL_TYPE] = SANDBOX

	if opts.Registry == "docker" {
		dm.codeMgr = newImageCodeManager(opts, c)
		dm.creator = newNamedImgSbCreator(c, labels, env)
	} else if opts.Registry == "olregistry" {
		dm.codeMgr = newRegistryCodeManager(opts)
		dm.creator = newBaseImgSbCreator(c, labels, env, opts.Reg_dir)
	} else if opts.Registry == "local" {
		dm.codeMgr = newLocalCodeManager(opts)
		dm.creator = newBaseImgSbCreator(c, labels, env, opts.Reg_dir)
	} else {
		log.Fatal("unrecognized registry type: %s", opts.Registry)
	}

	return dm
}

func (dm *DockerManager) Create(name string, sandbox_dir string) (sb.Sandbox, error) {
	return dm.creator.Create(name, sandbox_dir)
}

func (dm *DockerManager) Pull(name string) error {
	return dm.codeMgr.Pull(name)
}

func (dm *DockerManager) Client() *docker.Client {
	return dm.client
}

func (dm *DockerManager) Dump() {
	opts := docker.ListContainersOptions{All: true}
	containers, err := dm.client.ListContainers(opts)
	if err != nil {
		log.Fatal("Could not get container list")
	}
	log.Printf("=====================================\n")
	for idx, info := range containers {
		container, err := dm.client.InspectContainer(info.ID)
		if err != nil {
			log.Fatal("Could not get container")
		}

		log.Printf("CONTAINER %d: %v, %v, %v\n", idx,
			info.Image,
			container.ID[:8],
			container.State.String())
	}
}
