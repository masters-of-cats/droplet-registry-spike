package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	digest "github.com/opencontainers/go-digest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
)

var BuildOCIImage = cli.Command{
	Name:        "build-oci-image",
	Usage:       "build-oci-image <rootfs tar> <droplet tar>",
	Description: "Creates an OCI image manifest based on the droplet tars specified",
	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("build-oci-image")

		if ctx.NArg() != 2 {
			return errors.New(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage))
		}

		for _, tarPath := range ctx.Args() {
			if _, err := os.Stat(tarPath); err != nil {
				return errors.New(fmt.Sprintf("File %s does not exist", tarPath))
			}
		}

		// TODO stop hardcoding
		store := "/Users/pivotal/workspace/registry-experiment/store"

		builder := &imageBuilder{store: store}
		manifest, err := builder.buildOCIManifest(ctx.Args()[0], ctx.Args()[1])
		if err != nil {
			return err
		}

		manifestMarshalled, err := json.Marshal(manifest)
		if err != nil {
			return err
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, manifestMarshalled, "", "    "); err != nil {
			return err
		}

		fmt.Println(pretty.String())

		return nil
	},
}

type imageBuilder struct {
	store string
}

func (b *imageBuilder) buildOCIManifest(rootfsTar string, dropletTar string) (specsv1.Manifest, error) {
	if err := os.MkdirAll(b.store, 0700); err != nil {
		return specsv1.Manifest{}, err
	}

	var err error
	manifest := specsv1.Manifest{}
	manifest.SchemaVersion = 2
	if manifest.Layers, err = b.buildOCIManifestLayers(rootfsTar, dropletTar); err != nil {
		return specsv1.Manifest{}, err
	}

	return manifest, nil

}

func (b *imageBuilder) buildOCIManifestLayers(rootfsPath, dropletPath string) ([]specsv1.Descriptor, error) {
	rootfsLayer, err := b.buildRootfsLayer(rootfsPath)
	if err != nil {
		return nil, err
	}

	dropletLayer, err := b.buildDropletLayer(dropletPath)
	if err != nil {
		return nil, err
	}

	return []specsv1.Descriptor{rootfsLayer, dropletLayer}, nil
}

func (b *imageBuilder) buildRootfsLayer(rootfsPath string) (specsv1.Descriptor, error) {
	originalRootfs, err := os.Open(rootfsPath)
	if err != nil {
		return specsv1.Descriptor{}, err
	}
	defer originalRootfs.Close()
	rootfsInfo, err := originalRootfs.Stat()
	if err != nil {
		return specsv1.Descriptor{}, err
	}
	originalRootfsSize := rootfsInfo.Size()

	summer := sha256.New()
	if _, err := io.Copy(summer, originalRootfs); err != nil {
		return specsv1.Descriptor{}, err
	}
	checksum := hex.EncodeToString(summer.Sum(nil))

	if _, err := originalRootfs.Seek(0, 0); err != nil {
		return specsv1.Descriptor{}, err
	}

	destTarName := filepath.Join(b.store, checksum)
	// TODO EXCL is more appropriate and TRUNC here, but only after we implement
	// caching
	destFile, err := os.OpenFile(destTarName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return specsv1.Descriptor{}, err
	}
	defer destFile.Close()
	written, err := io.Copy(destFile, originalRootfs)
	if err != nil {
		return specsv1.Descriptor{}, err
	}
	if written != originalRootfsSize {
		return specsv1.Descriptor{}, fmt.Errorf("wrote %dB, expected to write %dB", written, originalRootfsSize)
	}

	layer := specsv1.Descriptor{}
	layer.MediaType = specsv1.MediaTypeImageLayerGzip
	layer.Size = originalRootfsSize
	layer.Digest = digest.Digest("sha256:" + checksum)

	return layer, nil

}

func (b *imageBuilder) buildDropletLayer(dropletPath string) (specsv1.Descriptor, error) {
	originalTar, err := os.Open(dropletPath)
	if err != nil {
		return specsv1.Descriptor{}, err
	}
	defer originalTar.Close()

	dropletInfo, err := originalTar.Stat()
	if err != nil {
		return specsv1.Descriptor{}, err
	}
	dropletSize := dropletInfo.Size()

	zipReader, err := gzip.NewReader(originalTar)
	if err != nil {
		return specsv1.Descriptor{}, err
	}
	tarReader := tar.NewReader(zipReader)

	destFile, err := ioutil.TempFile("", "oci-cli")
	if err != nil {
		return specsv1.Descriptor{}, err
	}

	summer := sha256.New()
	tee := io.MultiWriter(summer, destFile)

	zipWriter := gzip.NewWriter(tee)
	tarWriter := tar.NewWriter(zipWriter)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return specsv1.Descriptor{}, err
		}

		header.Name = "/home/vcap" + string(header.Name[1:])

		if err := tarWriter.WriteHeader(header); err != nil {
			return specsv1.Descriptor{}, err
		}

		if _, err := io.Copy(tarWriter, tarReader); err != nil {
			return specsv1.Descriptor{}, err
		}
	}
	destFile.Close()

	checksum := hex.EncodeToString(summer.Sum(nil))
	if err := os.Rename(destFile.Name(), filepath.Join(b.store, checksum)); err != nil {
		return specsv1.Descriptor{}, err
	}

	return specsv1.Descriptor{
		MediaType: specsv1.MediaTypeImageLayerGzip,
		Size:      dropletSize,
		Digest:    digest.Digest("sha256:" + checksum),
	}, nil
}
