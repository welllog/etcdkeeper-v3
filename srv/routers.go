package srv

import "net/http"

func bindV3Router(mux *http.ServeMux, v3 *v3Handlers) {
	mux.HandleFunc("GET /hosts", v3.Hosts)
	mux.HandleFunc("POST /v3/connect", v3.Connect)
	mux.HandleFunc("PUT /v3/put", v3.Put)
	mux.HandleFunc("GET /v3/get", v3.Get)
	mux.HandleFunc("POST /v3/delete", v3.Del)
	mux.HandleFunc("GET /v3/getpath", v3.GetPath)
}
