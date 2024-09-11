package main

import (
	"embed"
	"flag"
	"os"

	"github.com/welllog/etcdkeeper-v3/srv"
	"github.com/welllog/olog"
	"gopkg.in/yaml.v3"
)

var configFile = flag.String("c", "./config.yaml", "config file path")

//go:embed assets
var assets embed.FS

func main() {
	flag.Parse()

	olog.SetLoggerOptions(
		olog.WithLoggerEncode(olog.PLAIN),
		olog.WithLoggerTimeFormat("2006/01/02 15:04:05"),
		olog.WithLoggerCaller(false),
	)

	var cf srv.Conf
	loadConfFromFile(&cf, *configFile)
	cf.Init()

	olog.SetLevel(olog.GetLevelByString(cf.Loglevel))

	srv.NewServer(cf, assets).Start()
}

func loadConfFromFile(cf *srv.Conf, cfFile string) {
	b, err := os.ReadFile(cfFile)
	if err == nil {
		olog.Infof("load ./config.yaml content: \n%s", string(b))
		err = yaml.Unmarshal(b, &cf)
		if err != nil {
			olog.Fatalf("unmarshal config.yaml failed: %v", err)
		}
		return
	}

	if !os.IsNotExist(err) {
		olog.Fatalf("read ./config.yaml failed: %v", err)
	}

	olog.Infof("not found ./config.yaml, use default config")
}
