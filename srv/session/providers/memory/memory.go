package memory

import (
	"container/list"
	"sync"
	"time"

	"github.com/welllog/etcdkeeper-v3/srv/session"
)

var p = &Provider{
	sessions: make(map[string]*list.Element, 10),
	list:     list.New(),
}

type SessionStore struct {
	sid          string         // session id唯一标示
	timeAccessed int64          // 最后访问时间
	values       map[string]any // session里面存储的值
	lock         sync.Mutex
}

func (st *SessionStore) Set(key string, value any) error {
	st.lock.Lock()
	st.values[key] = value
	st.lock.Unlock()
	return nil
}

func (st *SessionStore) Get(key string) (any, bool) {
	st.lock.Lock()
	v, ok := st.values[key]
	st.lock.Unlock()
	return v, ok
}

func (st *SessionStore) Delete(key string) error {
	st.lock.Lock()
	delete(st.values, key)
	st.lock.Unlock()
	return nil
}

func (st *SessionStore) SessionID() string {
	return st.sid
}

type Provider struct {
	lock     sync.Mutex               // 用来锁
	sessions map[string]*list.Element // 用来存储在内存
	list     *list.List               // 用来做gc
}

func (pder *Provider) SessionInit(sid string) (session.Session, error) {
	pder.lock.Lock()
	sess, err := pder.sessionInit(sid)
	pder.lock.Unlock()

	return sess, err
}

func (pder *Provider) SessionRead(sid string) (session.Session, error) {
	pder.lock.Lock()
	defer pder.lock.Unlock()

	element, ok := pder.sessions[sid]
	if ok {
		sess := element.Value.(*SessionStore)
		sess.timeAccessed = time.Now().Unix()
		pder.list.MoveToBack(element)
		return sess, nil
	}

	return pder.sessionInit(sid)
}

func (pder *Provider) SessionDestroy(sid string) error {
	pder.lock.Lock()
	if element, ok := pder.sessions[sid]; ok {
		delete(pder.sessions, sid)
		pder.list.Remove(element)
	}
	pder.lock.Unlock()

	return nil
}

func (pder *Provider) SessionGC(maxlifetime int64) {
	now := time.Now().Unix()

	pder.lock.Lock()
	for {
		element := pder.list.Front()
		if element == nil {
			break
		}

		sess := element.Value.(*SessionStore)
		if sess.timeAccessed+maxlifetime > now {
			break
		}

		pder.list.Remove(element)
		delete(pder.sessions, sess.sid)
	}
	pder.lock.Unlock()
}

func (pder *Provider) sessionInit(sid string) (session.Session, error) {
	newsess := &SessionStore{
		sid:          sid,
		timeAccessed: time.Now().Unix(),
		values:       make(map[string]any, 2),
	}

	element := pder.list.PushBack(newsess)
	pder.sessions[sid] = element
	return newsess, nil
}

func init() {
	session.Register("memory", p)
}
