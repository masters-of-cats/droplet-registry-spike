package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

func main() {
	store := flag.String("store", "", "store")
	flag.Parse()

	httpHandler := mux.NewRouter()
	httpHandler.HandleFunc("/v2/{name}/manifests/{tag}", func(w http.ResponseWriter, r *http.Request) {
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
	}).Methods("GET")

	httpHandler.HandleFunc("/v2/{name}/blobs/{digest}", func(w http.ResponseWriter, r *http.Request) {
		pathParams := mux.Vars(r)
		blob, err := os.Open(filepath.Join(*store, pathParams["digest"]))
		if err != nil {
			panic(err)
		}
		defer blob.Close()
		if _, err := io.Copy(w, blob); err != nil {
			panic(err)
		}
	})

	server := negroni.Classic()
	server.UseHandler(httpHandler)
	server.Run(":8080") // TODO parameterise
}
