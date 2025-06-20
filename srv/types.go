package srv

import (
	"encoding/json"
	"net/http"
)

type userInfo struct {
	Host   string
	Name   string
	Passwd string
}

type Rsp map[string]any

func (r Rsp) WriteTo(w http.ResponseWriter) {
	JsonRsp(w, r)
}

func JsonRsp(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

type Node struct {
	Key           string  `json:"key"`
	Value         string  `json:"value,omitempty"`
	Dir           bool    `json:"dir,omitempty"`
	CreatedIndex  int64   `json:"createdIndex,omitempty"`
	ModifiedIndex int64   `json:"modifiedIndex,omitempty"`
	VersionIndex  int64   `json:"versionIndex,omitempty"`
	Ttl           int64   `json:"ttl,omitempty"`
	Nodes         []*Node `json:"nodes,omitempty"`
}

type NodeRsp struct {
	Node Node `json:"node"`
}

func (n NodeRsp) WriteTo(w http.ResponseWriter) {
	JsonRsp(w, n)
}

type NodesRsp struct {
	Nodes []*Node `json:"nodes"`
}

func (n NodesRsp) WriteTo(w http.ResponseWriter) {
	JsonRsp(w, n)
}

type HostInfo struct {
	Host string `json:"host"`
	Name string `json:"name"`
}

type keyRange struct {
	from string
	end  string
}
