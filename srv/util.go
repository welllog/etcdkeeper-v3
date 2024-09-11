package srv

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	KB = 1 << 10
	MB = 1 << 20
	GB = 1 << 30
)

func sizeFormat(size int64) string {
	if size > GB {
		return fmt.Sprintf("%sGB", formatFloat(float64(size)/GB))
	}

	if size > MB {
		return fmt.Sprintf("%sMB", formatFloat(float64(size)/MB))
	}

	if size > KB {
		return fmt.Sprintf("%sKB", formatFloat(float64(size)/KB))
	}

	return fmt.Sprintf("%dB", size)
}

func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	if s[len(s)-2:] == "00" {
		return s[:len(s)-3]
	}

	if s[len(s)-1:] == "0" {
		return s[:len(s)-1]
	}

	return s
}

func isEtcdServerErr(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "etcdserver:")
}

func newEtcdClient(name, passwd string, cf Etcd) (*clientv3.Client, error) {
	var tlsConfig *tls.Config
	if cf.Tls.Enable {
		tlsInfo := transport.TLSInfo{
			CertFile:      cf.Tls.CertFile,
			KeyFile:       cf.Tls.KeyFile,
			TrustedCAFile: cf.Tls.TrustedCAFile,
		}
		var err error
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("tls config failed: %w", err)
		}
	}

	conf := clientv3.Config{
		Endpoints:            []string{cf.Endpoints},
		DialTimeout:          10 * time.Second,
		TLS:                  tlsConfig,
		DialKeepAliveTime:    time.Minute,
		DialKeepAliveTimeout: time.Minute,
		Username:             name,
		Password:             passwd,
	}

	c, err := clientv3.New(conf)
	if err != nil {
		return nil, fmt.Errorf("etcd connect failed: %w", err)
	}

	return c, nil
}
