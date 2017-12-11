package main

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

const manifestMediaType = "application/vnd.docker.distribution.manifest.v2+json"

type api struct {
	*negroni.Negroni
	listenAddress string
	store         *storeManager
}

func NewAPI(listenAddress string, store *storeManager) *api {
	if listenAddress == "" {
		panic("please set --listen-address")
	}

	server := &api{listenAddress: listenAddress, Negroni: negroni.Classic(), store: store}
	httpHandler := mux.NewRouter()

	httpHandler.HandleFunc("/v2/", server.emptyBody).Methods("GET")
	httpHandler.HandleFunc("/v2/{app-guid}/manifests/{tag}", server.getManifest).Methods("GET")
	httpHandler.HandleFunc("/v2/{app-guid}/blobs/{digest}", server.getBlob).Methods("GET")

	server.UseHandler(httpHandler)
	return server
}

func (a *api) ListenAndServe() {
	a.Run(a.listenAddress)
}

func (a *api) emptyBody(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (a *api) getManifest(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	appGUID := pathParams["app-guid"]

	w.Header().Add("Content-Type", manifestMediaType)
	a.store.AppManifest(w, appGUID)
}

func (a *api) getBlob(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	blobDigest := pathParams["digest"]

	a.store.GetBlob(w, strings.Split(blobDigest, ":")[1])
}

type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

func layerDescriptor(digest string, size int64) descriptor {
	return descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Digest:    "sha256:" + digest,
		Size:      size,
	}
}

func configDescriptor(digest string, size int64) descriptor {
	return descriptor{
		MediaType: "application/vnd.docker.container.image.v1+json",
		Digest:    "sha256:" + digest,
		Size:      size,
	}
}

type imageConfig struct {
	ContainerConfig containerConfig `json:"config"`
	Rootfs          rootfs          `json:"rootfs"`
}

type containerConfig struct {
	User string `json:"user"`
}

type rootfs struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

func createImageConfig(diffIDs ...string) imageConfig {
	return imageConfig{
		ContainerConfig: containerConfig{User: "vcap"},
		Rootfs:          rootfs{Type: "layers", DiffIDs: diffIDs},
	}
}

type manifest struct {
	MediaType     string       `json:"mediaType"`
	SchemaVersion int          `json:"schemaVersion"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
}

func createManifest(config descriptor, layers ...descriptor) manifest {
	return manifest{
		MediaType:     manifestMediaType,
		SchemaVersion: 2,
		Config:        config,
		Layers:        layers,
	}
}
