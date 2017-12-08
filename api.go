package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

type api struct {
	*negroni.Negroni
	storePath     string
	listenAddress string
}

func NewAPI(listenAddress, storePath string) *api {
	if listenAddress == "" {
		panic("please set --listen-address")
	}
	if storePath == "" {
		panic("please set --store")
	}

	server := &api{listenAddress: listenAddress, Negroni: negroni.Classic(), storePath: storePath}
	httpHandler := mux.NewRouter()
	httpHandler.HandleFunc("/v2/{name}/manifests/{tag}", server.getManifest).Methods("GET")
	httpHandler.HandleFunc("/v2/{name}/blobs/{digest}", server.getLayer).Methods("GET")
	server.UseHandler(httpHandler)
	return server
}

func (a *api) ListenAndServe() {
	a.Run(a.listenAddress)
}

func (a *api) getManifest(w http.ResponseWriter, r *http.Request) {
	manifest := `{
   "name": "testytest",
   "tag": "tagg",
   "fsLayers": [
      {
				"blobSum": "sha256:8e6b0a5eff3664b2dc52fc1ddfa6692c1e0ca0acc8b8a257958657a78590118e"
      },
      {
				"blobSum": "sha256:080046d8de86bce1034dd89daeac2b467ca4185991a20b3d9039bbb7e7bb544f"
      }
   ],
	 "signature": "NOT_USED"
 }`
	fmt.Fprintln(w, manifest)
}

func (a *api) getLayer(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	blob, err := os.Open(filepath.Join(a.storePath, pathParams["digest"]))
	must("opening layer file from store", err)
	defer blob.Close()
	_, err = io.Copy(w, blob)
	must("copying layer over HTTP", err)
}
