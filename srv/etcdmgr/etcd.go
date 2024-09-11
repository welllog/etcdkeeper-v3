package etcdmgr

import (
	"container/list"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type etcdItem struct {
	key          string
	timeAccessed int64
	client       *clientv3.Client
}

type EtcdManager struct {
	clients    map[string]*list.Element
	list       *list.List
	maxIdeSecs int64
	mu         sync.Mutex
}

func NewEtcdManager(maxIdeSecs int64) *EtcdManager {
	mgr := &EtcdManager{
		clients:    make(map[string]*list.Element, 2),
		list:       list.New(),
		maxIdeSecs: maxIdeSecs,
	}

	go mgr.gc()

	return mgr
}

func (m *EtcdManager) GetClient(key string) (*clientv3.Client, bool) {
	m.mu.Lock()

	i, ok := m.clients[key]
	if ok {
		i.Value.(*etcdItem).timeAccessed = time.Now().Unix()
		m.list.MoveToBack(i)
	}

	m.mu.Unlock()

	if !ok {
		return nil, false
	}

	return i.Value.(*etcdItem).client, true
}

func (m *EtcdManager) SetClient(key string, c *clientv3.Client) {
	m.mu.Lock()

	oi, ok := m.clients[key]
	if ok {
		m.list.Remove(oi)
		// ? the client maybe used by other goroutine
		oi.Value.(*etcdItem).client.Close()
	}

	ele := m.list.PushBack(&etcdItem{
		key:          key,
		timeAccessed: time.Now().Unix(),
		client:       c,
	})
	m.clients[key] = ele

	m.mu.Unlock()
}

func (m *EtcdManager) SetClientNX(key string, c *clientv3.Client) bool {
	m.mu.Lock()

	_, ok := m.clients[key]
	if !ok {
		ele := m.list.PushBack(&etcdItem{
			key:          key,
			timeAccessed: time.Now().Unix(),
			client:       c,
		})
		m.clients[key] = ele
	}

	m.mu.Unlock()

	return !ok
}

func (m *EtcdManager) gc() {
	timer := time.NewTimer(time.Duration(m.maxIdeSecs) * time.Second)

	for {
		now := <-timer.C
		unix := now.Unix()

		m.mu.Lock()

		for {
			ele := m.list.Front()
			if ele == nil {
				break
			}

			i := ele.Value.(*etcdItem)
			if unix-i.timeAccessed < m.maxIdeSecs {
				break
			}

			m.list.Remove(ele)
			i.client.Close()
			delete(m.clients, i.key)
		}

		m.mu.Unlock()

		timer.Reset(time.Duration(m.maxIdeSecs) * time.Second)
	}
}
