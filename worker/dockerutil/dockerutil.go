package dockerutil

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
)

func ImageExists(client *docker.Client, name string) (bool, error) {
	_, err := client.InspectImage(name)
	if err == docker.ErrNoSuchImage {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func AssertImageExists(client *docker.Client, name string) error {
	if exists, err := ImageExists(client, name); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("Docker image %s does not exist", name)
	}
	return nil
}

func PullAndTag(client *docker.Client, registry string, img string) error {
	err := client.PullImage(
		docker.PullImageOptions{
			Repository: registry + "/" + img,
			Registry:   registry,
			Tag:        "latest",
		},
		docker.AuthConfiguration{},
	)

	if err != nil {
		return fmt.Errorf("failed to pull '%v' from %v registry\n", img, registry)
	}

	err = client.TagImage(
		registry+"/"+img,
		docker.TagImageOptions{Repo: img, Force: true})
	if err != nil {
		return fmt.Errorf("failed to re-tag container: %v\n", err)
	}

	return nil
}

func Create(client *docker.Client, img string, labels map[string]string, env []string, volumes []string) (*docker.Container, error) {
	// TODO: consider support port binding?
	return client.CreateContainer(
		docker.CreateContainerOptions{
			Config: &docker.Config{
				Image:  img,
				Labels: labels,
				Env:    env,
			},
			HostConfig: &docker.HostConfig{
				Binds: volumes,
			},
		},
	)
}
