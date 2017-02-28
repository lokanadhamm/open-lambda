package sbmanager

/*

Contains codeManager and its implementations. They are used by SandboxManager
to delegate the Pull() call. All are private since user should not care about
the implementation details, but only the expected bebaviors of the manager.

*/

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"
	r "github.com/open-lambda/open-lambda/registry/src"
	"github.com/open-lambda/open-lambda/worker/config"
	"github.com/open-lambda/open-lambda/worker/dockerutil"
)

// TODO: Should we have another function for checking if then code already exists?
type codeManager interface {
	Pull(name string) error
}

// imageCodeManager pulls Docker images that contains the handler codes from registry.
type imageCodeManager struct {
	registry  string
	client    *docker.Client
	skipExist bool
}

// localCodeManager loads handler code from local directory handler_dir.
type localCodeManager struct {
	handler_dir string
}

// registryCodeManager pulls handler codes from olstore using pull client and
// stores them locally in handler_dir.
type registryCodeManager struct {
	pullclient  *r.PullClient
	handler_dir string
}

// newImageCodeManager creates an imageCodeManager.
func newImageCodeManager(opts *config.Config, client *docker.Client) *imageCodeManager {
	im := new(imageCodeManager)
	im.registry = fmt.Sprintf("%s:%s", opts.Registry_host, opts.Registry_port)
	im.client = client
	im.skipExist = opts.Skip_pull_existing
	return im
}

// Pull checks if the image already exists, and skip, pull, or remove then pull
// depending on its setup.
func (im *imageCodeManager) Pull(name string) error {
	// delete if it exists, so we can pull a new one
	exists, err := dockerutil.ImageExists(im.client, name)
	if err != nil {
		return err
	}
	if exists {
		if im.skipExist {
			return nil
		}
		opts := docker.RemoveImageOptions{Force: true}
		if err := im.client.RemoveImageExtended(name, opts); err != nil {
			return err
		}
	}

	// pull new code
	if err := dockerutil.PullAndTag(im.client, im.registry, name); err != nil {
		return err
	}

	return nil
}

// newLocalCodeManager creates a localCodeManager.
func newLocalCodeManager(opts *config.Config) *localCodeManager {
	lm := new(localCodeManager)
	lm.handler_dir = opts.Reg_dir
	return lm
}

// Pull checks if the handler code exists locally.
func (lm *localCodeManager) Pull(name string) error {
	path := filepath.Join(lm.handler_dir, name)
	_, err := os.Stat(path)

	return err
}

// newRegistryCodeManager creates a registryCodeManager.
func newRegistryCodeManager(opts *config.Config) *registryCodeManager {
	rm := new(registryCodeManager)
	rm.pullclient = r.InitPullClient(opts.Reg_cluster, r.DATABASE, r.TABLE)
	// TODO: should we use other directory to store handler?
	rm.handler_dir = opts.Reg_dir
	return rm
}

// Pull uses pull client to pull code from registry and uncompresses to local.
func (rm *registryCodeManager) Pull(name string) error {
	dir := filepath.Join(rm.handler_dir, name)
	if err := os.Mkdir(dir, os.ModeDir); err != nil {
		return err
	}

	pfiles := rm.pullclient.Pull(name)
	handler := pfiles[r.HANDLER].([]byte)
	r := bytes.NewReader(handler)

	// TODO: try to uncompress without execing - faster?
	cmd := exec.Command("tar", "-xvzf", "-", "--directory", dir)
	cmd.Stdin = r
	return cmd.Run()
}
