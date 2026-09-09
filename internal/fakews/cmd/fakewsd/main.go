// fakewsd serves the recorded fake workspace on a fixed port so terraform and
// datatf can be pointed at it during local end-to-end checks:
//
//	go run ./internal/fakews/cmd/fakewsd -addr 127.0.0.1:8787
//	DATABRICKS_HOST=http://127.0.0.1:8787 DATABRICKS_TOKEN=fake DATABRICKS_AUTH_TYPE=pat datatf export ...
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/536tech/datatf/internal/fakews"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	flag.Parse()

	srv := fakews.NewStandalone()
	fmt.Printf("fakews listening on http://%s (workspace id %d)\n", *addr, fakews.WorkspaceID)
	go func() {
		if err := http.ListenAndServe(*addr, srv); err != nil {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
}
