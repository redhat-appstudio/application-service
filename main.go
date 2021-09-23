package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	restful "github.com/emicklei/go-restful/v3"
)

func main() {
	ws := new(restful.WebService)
	ws.
		Path("/api/v1/").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ws.Route(ws.GET("/").To(hello))

	// Set up routes for the app service API
	ws.Route(ws.POST("/application/create").To(hello))
	ws.Route(ws.POST("/application/push").To(hello))
	ws.Route(ws.GET("/samples").To(hello))
	ws.Route(ws.POST("/service/create").To(hello))

	restful.Add(ws)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func hello(req *restful.Request, resp *restful.Response) {
	_, err := io.WriteString(resp, "Hello")
	if err != nil {
		fmt.Println(err.Error())
	}
}
