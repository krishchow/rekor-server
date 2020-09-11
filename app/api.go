/*
Copyright © 2020 Luke Hinds <lhinds@redhat.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

// https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html
// https://github.com/dhax/go-base/blob/master/api/api.go
// curl http://localhost:3000/add -F "fileupload=@/tmp/file" -vvv

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/go-chi/chi"
	"github.com/google/trillian"
	"github.com/projectrekor/rekor-server/logging"
	"github.com/spf13/viper"
)

func ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "pong!")
}

func receiveHandler(w http.ResponseWriter, r *http.Request) {
	tLogID := viper.GetInt64("tlog_id")
	logRpcServer := viper.GetString("log_rpc_server")
	file, header, err := r.FormFile("fileupload")

	if err != nil {
		logging.Logger.Errorf("Error in r.FormFile ", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "{'error': %s}", err)
		return
	}
	defer file.Close()

	out, err := os.Create(header.Filename)
	if err != nil {
		logging.Logger.Errorf("Unable to create the file for writing. Check your write access privilege.", err)
		fmt.Fprintf(w, "Unable to create the file for writing. Check your write access privilege.", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		logging.Logger.Errorf("Error copying file.", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// return that we have successfully uploaded our file!
	fmt.Fprintf(w, "Successfully Uploaded File\n")
	logging.Logger.Info("Received file : ", header.Filename)

	// fetch an GPRC connection
	connection, err := dial(logRpcServer)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}

	leafFile, err := os.Open(header.Filename)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	byteLeaf, _ := ioutil.ReadAll(leafFile)
	defer leafFile.Close()

	tLogClient := trillian.NewTrillianLogClient(connection)
	server := serverInstance(tLogClient, tLogID)

	resp := &Response{}

	resp, err = server.addLeaf(byteLeaf, tLogID)
	logging.Logger.Infof("Server PUT Response: %s", resp.status)
	fmt.Fprintf(w, "Server PUT Response: %s", resp.status)

}

func New() (*chi.Mux, error) {
	router := chi.NewRouter()
	router.Post("/add", receiveHandler)
	router.Get("/ping", ping)
	return router, nil
}