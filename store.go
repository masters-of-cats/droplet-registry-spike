package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pborman/uuid"
)

var httpClient *http.Client

func init() {
	httpClient = &http.Client{Transport: &http.Transport{
		// SPIKE
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
}

type storeManager struct {
	path          string
	capiURL       string
	capiAuthToken string
	logger        *log.Logger

	rootfsDesc   descriptor
	rootfsDiffID string
}

func (s *storeManager) AppManifest(dest io.Writer, appGUID string) {
	s.logger.Printf("getting manifest for app %s...", appGUID)
	defer s.logger.Printf("done getting manifest for app %s", appGUID)

	// This spike doesn't support apps whose names are valid hex-encoded sha256
	cachedManifestPath := filepath.Join(s.path, appGUID+"-manifest")
	cachedManifestFile, err := os.Open(cachedManifestPath)
	if err == nil {
		s.logger.Println("manifest and associated layers already cached")
		_, err = io.Copy(dest, cachedManifestFile)
		must("copy cached manifest", err)
		cachedManifestFile.Close()
		return
	}
	if !os.IsNotExist(err) {
		must("should never happen", err)
	}

	appLayerDesc, appLayerDiffID := s.importAppLayer(appGUID)

	appConfig := createImageConfig(s.rootfsDiffID, appLayerDiffID)
	configJson, err := json.Marshal(appConfig)
	must("marshalling config", err)
	checksumBytes := sha256.Sum256(configJson)
	checksum := hex.EncodeToString(checksumBytes[:])
	must("write config json", ioutil.WriteFile(filepath.Join(s.path, checksum), configJson, 0600))
	configDesc := configDescriptor(checksum, int64(len(configJson)))

	manifest := createManifest(configDesc, s.rootfsDesc, appLayerDesc)

	cachedManifestFile, err = os.OpenFile(cachedManifestPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	must("opening manifest for writing", err)
	defer cachedManifestFile.Close()

	must("copy new manifest", json.NewEncoder(io.MultiWriter(dest, cachedManifestFile)).Encode(manifest))
}

func (s *storeManager) GetBlob(dest io.Writer, blobChecksum string) {
	blobFile, err := os.Open(filepath.Join(s.path, blobChecksum))
	must("open blob file", err)
	defer blobFile.Close()
	_, err = io.Copy(dest, blobFile)
	must("copy blob", err)
}

func uncompressedChecksum() (chan string, *os.File) {
	pipeR, pipeW, err := os.Pipe()
	must("mkpipes", err)
	uncompressedSummer := sha256.New()
	result := make(chan string)
	go func() {
		uncompressedReader, err := gzip.NewReader(pipeR)
		must("treat file as gzip", err)
		defer uncompressedReader.Close()
		_, err = io.Copy(uncompressedSummer, uncompressedReader)
		must("copy uncompressed file", err)
		result <- "sha256:" + hex.EncodeToString(uncompressedSummer.Sum(nil))
		pipeR.Close()
	}()

	return result, pipeW
}

func (s *storeManager) importRootfs(rootfsPath string) {
	s.logger.Printf("importing rootfs from %s...", rootfsPath)
	defer s.logger.Printf("done importing rootfs from %s", rootfsPath)

	must("create store", os.MkdirAll(s.path, 0700))

	originalRootfs, err := os.Open(rootfsPath)
	must("open rootfs", err)
	defer originalRootfs.Close()
	rootfsInfo, err := originalRootfs.Stat()
	must("stat rootfs", err)
	originalRootfsSize := rootfsInfo.Size()

	s.logger.Println("calculating rootfs compressed and uncompressed checksums...")
	summer := sha256.New()
	uncompressedChecksumResult, pipeW := uncompressedChecksum()

	tee := io.MultiWriter(pipeW, summer)

	_, err = io.Copy(tee, originalRootfs)
	must("checksum rootfs", err)
	pipeW.Close()
	diffID := <-uncompressedChecksumResult
	checksum := hex.EncodeToString(summer.Sum(nil))
	s.rootfsDesc = layerDescriptor(checksum, originalRootfsSize)
	s.rootfsDiffID = diffID

	storedRootfsPath := filepath.Join(s.path, checksum)
	_, err = os.Stat(storedRootfsPath)
	if err == nil {
		s.logger.Println("rootfs already cached")
		return
	}
	if !os.IsNotExist(err) {
		must("stat cached rootfs", err)
	}
	s.logger.Println("rootfs not cached, copying into store")

	_, err = originalRootfs.Seek(0, 0)
	must("seek rootfs back to 0", err)

	destFile, err := os.OpenFile(storedRootfsPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	must("open new rootfs file in store for writing", err)
	defer destFile.Close()
	_, err = io.Copy(destFile, originalRootfs)
	must("write rootfs file to store", err)
}

func (s *storeManager) downloadDroplet(appGUID string) string {
	s.logger.Printf("downloading droplet for app %s...", appGUID)
	defer s.logger.Printf("done downloading droplet for app %s", appGUID)

	dropletPath := filepath.Join(s.path, appGUID+"-droplet")
	_, err := os.Stat(dropletPath)
	if err == nil {
		return dropletPath
	}

	request, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/apps/%s/droplet/download", s.capiURL, appGUID), nil)
	must("create a request", err)
	request.Header.Add("Authorization", s.capiAuthToken) // "bearer" is already prefixed in the result of `cf oauth-token`

	response, err := httpClient.Do(request)
	must("do a request", err)
	defer response.Body.Close()

	file, err := os.Create(dropletPath)
	must("create app-droplet file", err)
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	must("write the droplet to a file", err)

	return dropletPath
}

func (s *storeManager) importAppLayer(appGUID string) (descriptor, string) {
	s.logger.Printf("getting layer for app %s...", appGUID)
	defer s.logger.Printf("done getting layer for app %s", appGUID)

	dropletPath := s.downloadDroplet(appGUID)
	dropletFile, err := os.Open(dropletPath)
	must("open droplet tarball", err)
	defer dropletFile.Close()

	zipReader, err := gzip.NewReader(dropletFile)
	must("assuming droplet is gzipped", err)
	tarReader := tar.NewReader(zipReader)

	destFile, err := os.Create(filepath.Join(s.path, uuid.New()))
	must("opening temporary file to re-tar droplet", err)
	summer := sha256.New()
	counter := new(byteCounter)
	tee := io.MultiWriter(summer, destFile, counter)
	zipWriter := gzip.NewWriter(tee)

	uncompressedSummer := sha256.New()

	tarWriter := tar.NewWriter(io.MultiWriter(zipWriter, uncompressedSummer))

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			must("droplet tar iteration error", err)
		}

		header.Name = filepath.Join("/home/vcap", header.Name)

		err = tarWriter.WriteHeader(header)
		must("write droplet tar header", err)

		_, err = io.Copy(tarWriter, tarReader)
		must("copy droplet tar entry", err)
	}
	must("close temporary droplet tarstream", tarWriter.Close())
	must("close temporary droplet zipper", zipWriter.Close())
	must("close temporary droplet file", destFile.Close())

	checksum := hex.EncodeToString(summer.Sum(nil))
	appLayerPath := filepath.Join(s.path, checksum)
	must("move droplet into store", os.Rename(destFile.Name(), appLayerPath))

	appLayerFile, err := os.Open(appLayerPath)
	must("opening app layer file", err)
	defer appLayerFile.Close()

	return layerDescriptor(checksum, counter.size), "sha256:" + hex.EncodeToString(uncompressedSummer.Sum(nil))
}
