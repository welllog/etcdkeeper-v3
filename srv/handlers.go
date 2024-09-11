package srv

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/welllog/etcdkeeper-v3/srv/etcdmgr"
	"github.com/welllog/etcdkeeper-v3/srv/session"
	"github.com/welllog/golib/strz"
	"github.com/welllog/olog"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type v3Handlers struct {
	conf    Conf
	sessmgr *session.Manager
	climgr  *etcdmgr.EtcdManager
}

func newV3Handlers(conf Conf) (*v3Handlers, error) {
	sessmgr, err := session.NewManager("memory", "_etcdkeeper_session", 3600)
	if err != nil {
		return nil, err
	}

	time.AfterFunc(86400*time.Second, func() {
		sessmgr.GC()
	})

	return &v3Handlers{
		conf:    conf,
		sessmgr: sessmgr,
		climgr:  etcdmgr.NewEtcdManager(3780),
	}, nil
}

func (h *v3Handlers) Hosts(w http.ResponseWriter, r *http.Request) {
	hosts := make([]HostInfo, len(h.conf.Etcds))
	for i := range h.conf.Etcds {
		hosts[i] = HostInfo{
			Host: h.conf.Etcds[i].Endpoints,
			Name: h.conf.Etcds[i].Name,
		}
	}

	Rsp{"hosts": hosts}.WriteTo(w)
}

func (h *v3Handlers) Connect(w http.ResponseWriter, r *http.Request) {
	sess := h.sessmgr.SessionStart(w, r)
	cuinfo := userInfo{
		Host:   r.FormValue("host"),
		Name:   r.FormValue("uname"),
		Passwd: r.FormValue("passwd"),
	}

	logger := olog.WithEntries(olog.GetLogger(), map[string]any{
		"method": r.Method,
		"host":   cuinfo.Host,
		"uname":  cuinfo.Name,
	})

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	value, ok := sess.Get(cuinfo.Host)
	if ok { // user login current etcd host
		uinfo := value.(*userInfo)

		if cuinfo.Name != "" && uinfo.Name != cuinfo.Name {
			// user switch
			goto login
		}

		cliKey := genCliKey(cuinfo.Host, uinfo.Name)
		cli, ok := h.climgr.GetClient(cliKey)
		if !ok {
			// current host client not exists
			goto login
		}

		info, err := h.getEtcdInfo(ctx, cli, cuinfo.Host)
		if err != nil {
			logger.Warnf("login user get etcd info err: %v", err)
			Rsp{"status": "error", "message": err.Error()}.WriteTo(w)
			return
		}

		_ = sess.Set("host", cuinfo.Host)
		Rsp{"status": "running", "info": info}.WriteTo(w)
		return
	}

login:
	cliKey := genCliKey(cuinfo.Host, cuinfo.Name)
	var closeNewCli bool
	cli, reuse := h.climgr.GetClient(cliKey)
	if !reuse {
		cf, ok := h.conf.GetEtcdConfig(cuinfo.Host)
		if !ok {
			cf.Endpoints = cuinfo.Host
		}

		var err error
		cli, err = newEtcdClient(cuinfo.Name, cuinfo.Passwd, cf)
		if err != nil {
			logger.Warnf("%s connect %s failed: %v", cuinfo.Name, cf.Endpoints, err)
			if isEtcdServerErr(err) {
				Rsp{"status": "login", "message": err.Error()}.WriteTo(w)
			} else {
				Rsp{"status": "error", "message": err.Error()}.WriteTo(w)
			}
			return
		}

		logger.Debugf("%s connect %s success", cuinfo.Name, cf.Endpoints)

		defer func() {
			if closeNewCli {
				_ = cli.Close()
			}
		}()

		// check password
		if cuinfo.Name == "" {
			// etcd may not open auth
			_, err = cli.AuthStatus(ctx)
		} else {
			// get user info to check password
			_, err = cli.UserGet(ctx, cuinfo.Name)
		}

		if err != nil {
			closeNewCli = true
			logger.Warnf("auth failed: %v", err)
			if isEtcdServerErr(err) {
				Rsp{"status": "login", "message": err.Error()}.WriteTo(w)
			} else {
				Rsp{"status": "error", "message": err.Error()}.WriteTo(w)
			}
			return
		}

	} else {
		// client already exists, check current user password
		if cli.Password != "" {
			if cuinfo.Passwd == "" {
				Rsp{"status": "login", "message": "Password required"}.WriteTo(w)
				return
			}

			if cli.Password != cuinfo.Passwd {
				_, err := cli.Authenticate(ctx, cuinfo.Name, cuinfo.Passwd)
				if err != nil {
					logger.Warnf("auth failed: %v", err)
					if isEtcdServerErr(err) {
						Rsp{"status": "login", "message": err.Error()}.WriteTo(w)
					} else {
						Rsp{"status": "error", "message": err.Error()}.WriteTo(w)
					}
					return
				}
			}
		}
	}

	info, err := h.getEtcdInfo(ctx, cli, cuinfo.Host)
	if err != nil {
		closeNewCli = true
		logger.Warnf("get etcd info err: %v", err)
		Rsp{"status": "error", "message": err.Error()}.WriteTo(w)
		return
	}

	// set login user info
	_ = sess.Set(cuinfo.Host, &cuinfo)
	_ = sess.Set("host", cuinfo.Host)
	// store client
	if !reuse {
		if !h.climgr.SetClientNX(cliKey, cli) {
			// client already exists, close the new client
			closeNewCli = true
		}
	}

	Rsp{"status": "running", "info": info}.WriteTo(w)
}

func (h *v3Handlers) Put(w http.ResponseWriter, r *http.Request) {
	cli, abort := h.getCli(w, r)
	if abort {
		return
	}

	key := r.FormValue("key")
	value := r.FormValue("value")
	ttl := r.FormValue("ttl")

	logger := olog.WithEntries(olog.GetLogger(), map[string]any{
		"method": r.Method,
		"host":   cli.Endpoints()[0],
		"uname":  cli.Username,
		"key":    key,
	})

	logger.Debug("PUT v3")

	ctx := r.Context()
	var putRsp *clientv3.PutResponse
	var err error
	var sec int64

	if ttl != "" {
		sec, err = strconv.ParseInt(ttl, 10, 64)
		if err != nil {
			logger.Warnf("parse ttl: %v", err)
			Rsp{"errorCode": 500, "message": "ttl parse failed: " + err.Error()}.WriteTo(w)
			return
		}

		var leaseResp *clientv3.LeaseGrantResponse
		leaseResp, err = cli.Grant(ctx, sec)
		if err != nil {
			logger.Warnf("grant lease failed: %v", err)
			Rsp{"errorCode": 500, "message": "grant lease failed: " + err.Error()}.WriteTo(w)
			return
		}

		putRsp, err = cli.Put(ctx, key, value, clientv3.WithLease(leaseResp.ID), clientv3.WithPrevKV())
	} else {
		putRsp, err = cli.Put(ctx, key, value, clientv3.WithPrevKV())
	}

	if err != nil {
		logger.Warnf("put failed: %v", err)
		Rsp{"errorCode": 500, "message": "put failed: " + err.Error()}.WriteTo(w)
		return
	}

	createdIndex := int64(0)
	modifiedIndex := int64(0)
	if putRsp.PrevKv != nil {
		createdIndex = putRsp.PrevKv.CreateRevision + 1
		modifiedIndex = putRsp.PrevKv.ModRevision + 1
	}

	NodeRsp{
		Node: Node{
			Key:           key,
			Value:         value,
			Ttl:           sec,
			CreatedIndex:  createdIndex,
			ModifiedIndex: modifiedIndex,
		},
	}.WriteTo(w)
}

func (h *v3Handlers) Get(w http.ResponseWriter, r *http.Request) {
	cli, abort := h.getCli(w, r)
	if abort {
		return
	}

	key := r.FormValue("key")
	withPrefix := r.FormValue("prefix") == "true"

	logger := olog.WithEntries(olog.GetLogger(), map[string]any{
		"method": r.Method,
		"host":   cli.Endpoints()[0],
		"uname":  cli.Username,
		"key":    key,
	})
	logger.Debug("GET v3")

	ctx := r.Context()
	if !withPrefix {
		getRsp, err := cli.Get(ctx, key)
		if err != nil {
			logger.Warnf("get failed: %v", err)
			Rsp{"errorCode": 500, "message": "get failed: " + err.Error()}.WriteTo(w)
			return
		}

		if len(getRsp.Kvs) == 0 {
			Rsp{"errorCode": 500, "message": "The key does not exist."}.WriteTo(w)
			return
		}

		var ttl int64
		if getRsp.Kvs[0].Lease > 0 {
			leaseRsp, err := cli.TimeToLive(ctx, clientv3.LeaseID(getRsp.Kvs[0].Lease))
			if err != nil {
				logger.Warnf("get lease failed: %v", err)
				Rsp{"errorCode": 500, "message": "get lease failed: " + err.Error()}.WriteTo(w)
				return
			}

			ttl = leaseRsp.TTL
		}
		NodeRsp{
			Node: Node{
				Key:           key,
				Value:         strz.UnsafeString(getRsp.Kvs[0].Value),
				Ttl:           ttl,
				CreatedIndex:  getRsp.Kvs[0].CreateRevision,
				ModifiedIndex: getRsp.Kvs[0].ModRevision,
			},
		}.WriteTo(w)
		return
	}

	getRsp := &clientv3.GetResponse{}
	var err error
	if cli.Username == "" || cli.Username == "root" {
		getRsp, err = cli.Get(ctx, key,
			clientv3.WithPrefix(),
			clientv3.WithKeysOnly(),
		)
	} else {
		keyRanges, err := getPermissionKeys(ctx, cli)
		if err != nil {
			logger.Warnf("get permission keys failed: %v", err)
			Rsp{"errorCode": 500, "message": "get permission keys failed: " + err.Error()}.WriteTo(w)
			return
		}

		keyRanges = slices.DeleteFunc(keyRanges, func(kr keyRange) bool {
			return !strings.HasPrefix(kr.from, key)
		})

		for _, kr := range keyRanges {
			rsp, err := cli.Get(ctx, kr.from,
				clientv3.WithFromKey(),
				clientv3.WithRange(kr.end),
				clientv3.WithKeysOnly(),
			)
			if err != nil {
				logger.Warnf("range get failed: %v", err)
				Rsp{"errorCode": 500, "message": "range get failed: " + err.Error()}.WriteTo(w)
				return
			}

			getRsp.Kvs = append(getRsp.Kvs, rsp.Kvs...)
		}
	}

	if err != nil {
		logger.Warnf("get failed: %v", err)
		Rsp{"errorCode": 500, "message": "get failed: " + err.Error()}.WriteTo(w)
		return
	}

	nodes := make([]*Node, len(getRsp.Kvs))
	for i, kv := range getRsp.Kvs {
		nodes[i] = &Node{
			Key:           string(kv.Key),
			CreatedIndex:  kv.CreateRevision,
			ModifiedIndex: kv.ModRevision,
		}
	}

	NodesRsp{Nodes: nodes}.WriteTo(w)
}

func (h *v3Handlers) Del(w http.ResponseWriter, r *http.Request) {
	cli, abort := h.getCli(w, r)
	if abort {
		return
	}

	key := r.FormValue("key")
	dir := r.FormValue("dir")

	logger := olog.WithEntries(olog.GetLogger(), map[string]any{
		"method": r.Method,
		"host":   cli.Endpoints()[0],
		"uname":  cli.Username,
		"key":    key,
	})

	logger.Debug("DELETE v3")

	ctx := r.Context()
	if _, err := cli.Delete(ctx, key); err != nil {
		_, _ = io.WriteString(w, err.Error())
		return
	}

	if dir == "true" {
		if _, err := cli.Delete(ctx, key, clientv3.WithPrefix()); err != nil {
			_, _ = io.WriteString(w, err.Error())
			return
		}
	}

	_, _ = io.WriteString(w, "ok")
}

func (h *v3Handlers) GetPath(w http.ResponseWriter, r *http.Request) {
	cli, abort := h.getCli(w, r)
	if abort {
		return
	}

	key := r.FormValue("key")

	logger := olog.WithEntries(olog.GetLogger(), map[string]any{
		"method": r.Method,
		"host":   cli.Endpoints()[0],
		"uname":  cli.Username,
		"key":    key,
	})

	logger.Debug("GET v3")

	getRsp := &clientv3.GetResponse{}
	var err error
	ctx := r.Context()
	if cli.Username == "" || cli.Username == "root" {
		getRsp, err = cli.Get(ctx, key,
			clientv3.WithPrefix(),
			clientv3.WithKeysOnly(),
		)
	} else {
		keyRanges, err := getPermissionKeys(ctx, cli)
		if err != nil {
			logger.Warnf("get permission keys failed: %v", err)
			Rsp{"errorCode": 500, "message": "get permission keys failed: " + err.Error()}.WriteTo(w)
			return
		}

		keyRanges = slices.DeleteFunc(keyRanges, func(kr keyRange) bool {
			return !strings.HasPrefix(kr.from, key)
		})

		for _, kr := range keyRanges {
			rsp, err := cli.Get(ctx, kr.from,
				clientv3.WithFromKey(),
				clientv3.WithRange(kr.end),
				clientv3.WithKeysOnly(),
			)
			if err != nil {
				logger.Warnf("range get failed: %v", err)
				Rsp{"errorCode": 500, "message": "range get failed: " + err.Error()}.WriteTo(w)
				return
			}

			getRsp.Kvs = append(getRsp.Kvs, rsp.Kvs...)
		}

	}

	if err != nil {
		logger.Warnf("get failed: %v", err)
		Rsp{"errorCode": 500, "message": "get failed: " + err.Error()}.WriteTo(w)
		return
	}

	cf, ok := h.conf.GetEtcdConfig(cli.Endpoints()[0])
	if !ok {
		cf.Separator = "/"
	}

	nodes, _ := buildNodes([]byte(key), []byte(cf.Separator), 0, getRsp.Kvs)
	NodesRsp{Nodes: nodes}.WriteTo(w)
}

func (h *v3Handlers) getEtcdInfo(ctx context.Context, cli *clientv3.Client, host string) (map[string]string, error) {
	stRsp, err := cli.Status(ctx, host)
	if err != nil {
		return nil, err
	}

	mbRsp, err := cli.MemberList(ctx)
	if err != nil {
		return nil, err
	}

	info := make(map[string]string, 3)
	info["version"] = stRsp.Version
	info["sizeInUse"] = sizeFormat(stRsp.DbSizeInUse)
	info["size"] = sizeFormat(stRsp.DbSize)

	for _, m := range mbRsp.Members {
		if m.ID == stRsp.Leader {
			info["name"] = m.GetName()
			break
		}
	}

	return info, nil
}

func (h *v3Handlers) getCli(w http.ResponseWriter, r *http.Request) (*clientv3.Client, bool) {
	abortRsp := Rsp{"errorCode": 500, "message": "Please connect to etcd first"}

	sess := h.sessmgr.SessionStart(w, r)
	host, ok := sess.Get("host")
	if !ok {
		abortRsp.WriteTo(w)
		return nil, true
	}

	infoValue, ok := sess.Get(host.(string))
	if !ok {
		abortRsp.WriteTo(w)
		return nil, true
	}

	cliKey := genCliKey(host.(string), infoValue.(*userInfo).Name)
	cli, ok := h.climgr.GetClient(cliKey)
	if !ok {
		abortRsp.WriteTo(w)
		return nil, true
	}

	return cli, false
}

func genCliKey(host, user string) string {
	return fmt.Sprintf("%s-%s", host, user)
}

func getPermissionKeys(ctx context.Context, cli *clientv3.Client) ([]keyRange, error) {
	ursp, err := cli.UserGet(ctx, cli.Username)
	if err != nil {
		return nil, fmt.Errorf("get user failed: %w", err)
	}

	keys := make([]keyRange, 0, 4)
	for _, role := range ursp.Roles {
		rrsp, err := cli.RoleGet(ctx, role)
		if err != nil {
			return nil, fmt.Errorf("get role failed: %w", err)
		}

		for _, p := range rrsp.Perm {
			if p.PermType == clientv3.PermReadWrite || p.PermType == clientv3.PermRead {
				keys = append(keys, keyRange{
					from: string(p.Key),
					end:  string(p.RangeEnd),
				})
			}
		}
	}

	return keys, nil
}

func buildNodes(prefix, separator []byte, idx int, kvs []*mvccpb.KeyValue) ([]*Node, int) {
	var nodes []*Node
	i := idx
	for i < len(kvs) {
		kv := kvs[i]
		if !bytes.HasPrefix(kv.Key, prefix) {
			return nodes, i - idx
		}

		rest := kv.Key[len(prefix):]
		offset := bytes.Index(rest, separator)
		if offset < 0 || offset == len(rest)-1 {
			if len(rest) > 0 {
				nodes = append(nodes, &Node{
					Key:           strz.UnsafeString(kv.Key),
					CreatedIndex:  kv.CreateRevision,
					ModifiedIndex: kv.ModRevision,
				})
			}
			i++
			continue
		}

		offset = offset + len(prefix) + 1
		node := &Node{
			Key: strz.UnsafeString(kv.Key[:offset]),
			Dir: true,
		}
		children, n := buildNodes(kv.Key[:offset], separator, i, kvs)
		node.Nodes = children
		nodes = append(nodes, node)

		i += n
	}

	return nodes, i
}
