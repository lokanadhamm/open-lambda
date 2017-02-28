package sbmanager

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

type imageCodeManager struct {
	registry  string
	client    *docker.Client
	skipExist bool
}

type localCodeManager struct {
	handler_dir string
}

type registryCodeManager struct {
	pullclient  *r.PullClient
	handler_dir string
}

func newImageCodeManager(opts *config.Config, client *docker.Client) *imageCodeManager {
	im := new(imageCodeManager)
	im.registry = fmt.Sprintf("%s:%s", opts.Registry_host, opts.Registry_port)
	im.client = client
	im.skipExist = opts.Skip_pull_existing
	return im
}

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

func newLocalCodeManager(opts *config.Config) *localCodeManager {
	lm := new(localCodeManager)
	lm.handler_dir = opts.Reg_dir
	return lm
}

func (lm *localCodeManager) Pull(name string) error {
	path := filepath.Join(lm.handler_dir, name)
	_, err := os.Stat(path)

	return err
}

func newRegistryCodeManager(opts *config.Config) *registryCodeManager {
	rm := new(registryCodeManager)
	rm.pullclient = r.InitPullClient(opts.Reg_cluster, r.DATABASE, r.TABLE)
	// TODO: should we use other directory to store handler?
	rm.handler_dir = opts.Reg_dir
	return rm
}

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
