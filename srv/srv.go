package srv

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pkg/browser"
	_ "github.com/welllog/etcdkeeper-v3/srv/session/providers/memory"
	"github.com/welllog/olog"
)

type Server struct {
	srv   http.Server
	debug bool
}

func NewServer(cf Conf, assets fs.FS) *Server {
	front, err := fs.Sub(assets, "assets")
	if err != nil {
		olog.Fatalf("sub assets: %v", err)
	}

	v3, err := newV3Handlers(cf)
	if err != nil {
		olog.Fatalf("new v3 handlers: %v", err)
	}

	mux := http.NewServeMux()
	if cf.Debug {
		mux.Handle("GET /", http.FileServer(http.Dir("./assets")))
		http.FileServerFS(front)
	} else {
		mux.Handle("GET /", http.FileServerFS(front))
	}

	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})
	bindV3Router(mux, v3)

	return &Server{
		srv:   http.Server{Addr: cf.Host + ":" + strconv.Itoa(cf.Port), Handler: mux},
		debug: cf.Debug,
	}
}

func (s *Server) Start() {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			olog.Fatalf("http listen %s error: %v", s.srv.Addr, err)
		}
	}()

	go func() {
		uri := fmt.Sprintf("http://%s/ping", s.srv.Addr)
		for i := 0; i < 15; i++ {
			resp, err := http.Get(uri)
			if err == nil && resp.StatusCode == 200 {
				olog.Infof("http server is ready on %s", s.srv.Addr)
				if os.Getenv("ETCDKEEPER_X_NO_BROWSER") == "" && !s.debug {
					if err := browser.OpenURL("http://" + s.srv.Addr); err != nil {
						olog.Warnf("open browser: %v", err)
					}
				}
				return
			}

			olog.Debug("waiting for the router, retry in 1 second.")
			time.Sleep(time.Second)
		}

		olog.Fatal("http server is not ready.")
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		olog.Fatalf("http server shutdown err: %s", err.Error())
	}

	olog.Info("http server shutdown")
}
